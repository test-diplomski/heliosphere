package handlers

import (
	"context"
	pb "rate-limiter-service/proto/ratelimiter"
	rs "rate-limiter-service/store"
)

type RateLimiterHandler struct {
	pb.UnimplementedRateLimitServiceServer
	store *rs.RateLimiterStore
}

func NewRateLimiterHandler() (*RateLimiterHandler, error) {
	store, err := rs.New()
	if err != nil {
		return nil, err
	}

	return &RateLimiterHandler{
		store: store,
	}, nil
}

func (h *RateLimiterHandler) CreateRateLimiter(ctx context.Context, request *pb.CreateRateLimiterRequest) (*pb.RateLimiter, error) {
	return h.store.Create(ctx, request)
}

func (h *RateLimiterHandler) GetRateLimiter(ctx context.Context, request *pb.GetRateLimiterRequest) (*pb.RateLimiter, error) {
	return h.store.Get(ctx, request.Id)
}

func (h *RateLimiterHandler) GetAllRateLimiters(ctx context.Context, request *pb.EmptyRequest) (*pb.ListOfRateLimiters, error) {
	return h.store.GetAll(ctx)
}

func (h *RateLimiterHandler) UpdateRateLimiter(ctx context.Context, request *pb.UpdateRateLimiterRequest) (*pb.RateLimiter, error) {
	return h.store.Update(ctx, request)
}

func (h *RateLimiterHandler) DeleteRateLimiter(ctx context.Context, request *pb.DeleteRateLimiterRequest) (*pb.DeleteRateLimiterResponse, error) {
	return h.store.Delete(ctx, request.Id)
}

func (h *RateLimiterHandler) CanRateLimiterAllowRequest(ctx context.Context, request *pb.AllowRequest) (*pb.AllowResponse, error) {
	return h.store.IsRequestAllowed(ctx, request.Id)
}
