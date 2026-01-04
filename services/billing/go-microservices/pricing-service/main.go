package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/billing-project/go-microservices/pricing-service/internal/proto"
	"github.com/billing-project/go-microservices/pricing-service/internal/services"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	defaultPort = "50051"
)

type server struct {
	pb.UnimplementedPricingServiceServer
	pricingService *services.PricingService
}

func (s *server) GetPricing(ctx context.Context, req *pb.GetPricingRequest) (*pb.GetPricingResponse, error) {
	log.Printf("Получен запрос на получение цен для модели: %s", req.ModelId)

	prices, err := s.pricingService.GetPricing(req.ModelId)
	if err != nil {
		log.Printf("Ошибка при получении цен: %v", err)
		return nil, fmt.Errorf("ошибка получения цен: %w", err)
	}

	return &pb.GetPricingResponse{
		ModelId: req.ModelId,
		Prices:  prices,
	}, nil
}

func (s *server) UpdatePricing(ctx context.Context, req *pb.UpdatePricingRequest) (*pb.UpdatePricingResponse, error) {
	log.Printf("Получен запрос на обновление цен для модели: %s", req.ModelId)

	err := s.pricingService.UpdatePricing(req.ModelId, req.Prices)
	if err != nil {
		log.Printf("Ошибка при обновлении цен: %v", err)
		return nil, fmt.Errorf("ошибка обновления цен: %w", err)
	}

	return &pb.UpdatePricingResponse{
		Success:   true,
		Message:   "Цены успешно обновлены",
		ModelId:   req.ModelId,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}, nil
}

func (s *server) ListModels(ctx context.Context, req *pb.ListModelsRequest) (*pb.ListModelsResponse, error) {
	log.Printf("Получен запрос на получение списка моделей")

	models, err := s.pricingService.ListModels()
	if err != nil {
		log.Printf("Ошибка при получении списка моделей: %v", err)
		return nil, fmt.Errorf("ошибка получения списка моделей: %w", err)
	}

	return &pb.ListModelsResponse{
		Models: models,
		Count:  int32(len(models)),
	}, nil
}

func (s *server) GetBulkPricing(ctx context.Context, req *pb.GetBulkPricingRequest) (*pb.GetBulkPricingResponse, error) {
	log.Printf("Получен запрос на получение цен для %d моделей", len(req.ModelIds))

	pricingMap := make(map[string]*pb.Pricing)
	for _, modelID := range req.ModelIds {
		prices, err := s.pricingService.GetPricing(modelID)
		if err != nil {
			log.Printf("Предупреждение: не удалось получить цены для модели %s: %v", modelID, err)
			// Продолжаем для других моделей, не прерываем весь запрос
			continue
		}
		pricingMap[modelID] = prices
	}

	return &pb.GetBulkPricingResponse{
		Pricing: pricingMap,
		Count:   int32(len(pricingMap)),
	}, nil
}

func (s *server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("Получен запрос health check")

	healthy := s.pricingService.IsHealthy()
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	return &pb.HealthCheckResponse{
		Service: "pricing-service",
		Status:  status,
		Version: "1.0.0",
		Time:    time.Now().Format(time.RFC3339),
	}, nil
}

func main() {
	// Получение порта из переменной окружения
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Инициализация сервиса цен
	pricingService, err := services.NewPricingService()
	if err != nil {
		log.Fatalf("Не удалось инициализировать сервис цен: %v", err)
	}
	defer pricingService.Close()

	// Создание gRPC сервера
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Не удалось создать listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPricingServiceServer(grpcServer, &server{
		pricingService: pricingService,
	})

	// Включение reflection для gRPC инструментов
	reflection.Register(grpcServer)

	log.Printf("Pricing Service запущен на порту %s", port)

	// Каналы для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Запуск сервера в горутине
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Ошибка при запуске сервера: %v", err)
		}
	}()

	log.Println("Pricing Service готов к обработке запросов")

	// Ожидание сигнала для graceful shutdown
	<-quit
	log.Println("Получен сигнал завершения...")

	grpcServer.GracefulStop()
	log.Println("Pricing Service остановлен")
}
