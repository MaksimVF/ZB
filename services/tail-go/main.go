package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yourorg/head/gen"
	"google.golang.org/grpc"
)

var headAddr = "localhost:50055"

type ChatMessage struct { Role string `json:"role"`; Content string `json:"content"` }
type ChatRequestIn struct {
	Model string `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Temperature float32 `json:"temperature"`
	MaxTokens int `json:"max_tokens"`
	Stream bool `json:"stream"`
	RequestId string `json:"request_id"`
}

func main() {
	if v:=os.Getenv("HEAD_ADDR"); v!="" { headAddr=v }
	http.HandleFunc("/v1/chat/completions", handleChat)
	log.Println("tail http on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequestIn
	body, _ := io.ReadAll(r.Body); _ = json.Unmarshal(body,&req)
	// build grpc request
	conn, err := grpc.Dial(headAddr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err!=nil { http.Error(w,"upstream error",502); return }
	defer conn.Close()
	cli := gen.NewChatServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	grpcReq := &gen.ChatRequest{ RequestId: req.RequestId, Model:req.Model, Temperature:req.Temperature, MaxTokens:int32(req.MaxTokens)}
	for _,m := range req.Messages { grpcReq.Messages = append(grpcReq.Messages, &gen.ChatMessage{Role:m.Role, Content:m.Content}) }
	resp, err := cli.ChatCompletion(ctx, grpcReq)
	if err!=nil { http.Error(w,"upstream error",502); return }
	out,_ := json.Marshal(map[string]interface{}{"request_id":resp.RequestId,"text":resp.FullText,"provider":resp.Provider,"tokens_used":resp.TokensUsed})
	w.Header().Set("Content-Type","application/json"); w.Write(out)
}
