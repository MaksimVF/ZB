package grpc

import (
	"context"

	"github.com/MaksimVF/ZB/services/secrets-service/core"
	pb "github.com/MaksimVF/ZB/services/secrets-service/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCHandler представляет gRPC обработчик для сервиса секретов
type GRPCHandler struct {
	pb.UnimplementedSecretServiceServer
	secretService *core.SecretService
	logger        *Logger
}

// NewGRPCHandler создает новый gRPC обработчик
func NewGRPCHandler(secretService *core.SecretService, logger *Logger) *GRPCHandler {
	return &GRPCHandler{
		secretService: secretService,
		logger:        logger,
	}
}

// GetSecret обрабатывает gRPC запрос на получение секрета
func (h *GRPCHandler) GetSecret(ctx context.Context, req *pb.GetSecretRequest) (*pb.GetSecretResponse, error) {
	h.logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Received GetSecret request")

	// Получение секрета через core сервис
	value, err := h.secretService.GetSecret(ctx, req.Name)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("secret_name", req.Name).
			Msg("Failed to get secret")

		return nil, h.handleError(err, "get_secret")
	}

	h.logger.Info().
		Str("method", "GetSecret").
		Str("secret_name", req.Name).
		Msg("Secret retrieved successfully")

	return &pb.GetSecretResponse{Value: value}, nil
}

// GetUserSecret обрабатывает gRPC запрос на получение пользовательского секрета
func (h *GRPCHandler) GetUserSecret(ctx context.Context, req *pb.GetUserSecretRequest) (*pb.GetUserSecretResponse, error) {
	h.logger.Info().
		Str("method", "GetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("Received GetUserSecret request")

	// Получение пользовательского секрета через core сервис
	value, err := h.secretService.GetUserSecret(ctx, req.UserId, req.SecretName)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("user_id", req.UserId).
			Str("secret_name", req.SecretName).
			Msg("Failed to get user secret")

		return nil, h.handleError(err, "get_user_secret")
	}

	h.logger.Info().
		Str("method", "GetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("User secret retrieved successfully")

	return &pb.GetUserSecretResponse{Value: value}, nil
}

// SetUserSecret обрабатывает gRPC запрос на установку пользовательского секрета
func (h *GRPCHandler) SetUserSecret(ctx context.Context, req *pb.SetUserSecretRequest) (*pb.SetUserSecretResponse, error) {
	h.logger.Info().
		Str("method", "SetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("Received SetUserSecret request")

	// Установка пользовательского секрета через core сервис
	err := h.secretService.SetUserSecret(ctx, req.UserId, req.SecretName, req.SecretValue)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("user_id", req.UserId).
			Str("secret_name", req.SecretName).
			Msg("Failed to set user secret")

		return nil, h.handleError(err, "set_user_secret")
	}

	h.logger.Info().
		Str("method", "SetUserSecret").
		Str("user_id", req.UserId).
		Str("secret_name", req.SecretName).
		Msg("User secret saved successfully")

	return &pb.SetUserSecretResponse{Status: "saved"}, nil
}

// handleError обрабатывает ошибки и преобразует их в gRPC статус коды
func (h *GRPCHandler) handleError(err error, operation string) error {
	h.logger.Error().Err(err).Str("operation", operation).Msg("Operation failed")

	// Проверяем тип ошибки и преобразуем в соответствующий gRPC статус
	switch {
	case containsError(err, "not found"):
		return status.Errorf(codes.NotFound, "secret not found: %v", err)
	case containsError(err, "permission denied"):
		return status.Errorf(codes.PermissionDenied, "permission denied: %v", err)
	case containsError(err, "invalid"):
		return status.Errorf(codes.InvalidArgument, "invalid argument: %v", err)
	default:
		return status.Errorf(codes.Internal, "internal server error: %v", err)
	}
}

// containsError проверяет, содержит ли ошибка указанный текст
func containsError(err error, text string) bool {
	return err != nil && len(err.Error()) > 0 && contains(err.Error(), text)
}

// contains проверяет, содержит ли строка подстроку
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
