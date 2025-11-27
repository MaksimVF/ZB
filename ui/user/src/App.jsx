



import React,{useState} from "react";

export default function App(){
  const [p,setP]=useState("");
  const [r,setR]=useState("");

  async function send(){
    const res=await fetch("/v1/chat/completions",{
      method:"POST",
      headers:{"Content-Type":"application/json"},
      body:JSON.stringify({model:"gpt-4o", messages:[{role:"user", content:p}]})
    });
    const j=await res.json();
    setR(JSON.stringify(j,null,2));
  }

  return <div style={{padding:20}}>
    <h1>User UI</h1>
    <textarea value={p} onChange={e=>setP(e.target.value)} rows={6} cols={80}></textarea><br/>
    <button onClick={send}>Send</button>
    <pre>{r}</pre>
  </div>
}



