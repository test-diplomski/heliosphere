package store

import (
	"context"
	"fmt"
	"log"
	"rate-limiter-service/config"
	pb "rate-limiter-service/proto/ratelimiter"
	"time"

	"github.com/RussellLuo/slidingwindow"
	"github.com/hashicorp/consul/api"
	"go.uber.org/ratelimit"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type RateLimiterStore struct {
	cli          *api.Client
	rateLimiters map[string]interface{}
}

func New() (*RateLimiterStore, error) {
	cfg := config.GetConfig()
	db := cfg.DB
	dbport := cfg.DBPort

	config := api.DefaultConfig()
	config.Address = fmt.Sprintf("%s:%s", db, dbport)
	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &RateLimiterStore{
		cli:          client,
		rateLimiters: make(map[string]interface{}),
	}, nil
}

func (rs *RateLimiterStore) Get(ctx context.Context, id string) (*pb.RateLimiter, error) {
	kv := rs.cli.KV()

	key := fmt.Sprintf("rateLimiters/%s", id)

	pair, _, err := kv.Get(key, nil)
	if err != nil || pair == nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Ratelimiter %s not found.", id))
	}

	rateLimiter := &pb.RateLimiter{}

	err = proto.Unmarshal(pair.Value, rateLimiter)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Ratelimiter %s cannot be unmarshaled.", id))
	}

	return rateLimiter, nil
}

func (rs *RateLimiterStore) GetAll(ctx context.Context) (*pb.ListOfRateLimiters, error) {
	kv := rs.cli.KV()
	data, _, err := kv.List("rateLimiters", nil)
	if err != nil {
		return nil, err
	}

	rateLimiters := []*pb.RateLimiter{}
	for _, pair := range data {
		rateLimiter := &pb.RateLimiter{}
		err = proto.Unmarshal(pair.Value, rateLimiter)
		if err != nil {
			return nil, err
		}
		rateLimiters = append(rateLimiters, rateLimiter)
	}

	return &pb.ListOfRateLimiters{
		Limiters: rateLimiters,
	}, nil
}

func (rs *RateLimiterStore) Create(ctx context.Context, limiter *pb.CreateRateLimiterRequest) (*pb.RateLimiter, error) {
	kv := rs.cli.KV()

	id := ""
	if limiter.RateLimiter.Name == "system" {
		id = fmt.Sprintf("%s", limiter.RateLimiter.Name)
	} else {
		id = fmt.Sprintf("%s-%s", limiter.RateLimiter.Name, limiter.RateLimiter.UserName)
	}
	limiter.RateLimiter.Id = id

	key := fmt.Sprintf("rateLimiters/%s", id)

	data, err := proto.Marshal(limiter.RateLimiter)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Ratelimiter %s cannot be created.", id))
	}

	p := &api.KVPair{Key: key, Value: data}
	_, err = kv.Put(p, nil)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Ratelimiter %s cannot be created.", id))
	}

	return limiter.RateLimiter, nil

}

func (rs *RateLimiterStore) Update(ctx context.Context, limiter *pb.UpdateRateLimiterRequest) (*pb.RateLimiter, error) {
	kv := rs.cli.KV()

	id := limiter.RateLimiter.Id

	key := fmt.Sprintf("rateLimiters/%s", id)

	data, err := proto.Marshal(limiter.RateLimiter)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Ratelimiter %s cannot be updated.", id))
	}

	p := &api.KVPair{Key: key, Value: data}
	_, err = kv.Put(p, nil)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("Ratelimiter %s cannot be updated.", id))
	}

	//update current state in map
	rs.createOrUpdateLimiter(limiter.RateLimiter, true)

	return limiter.RateLimiter, nil
}

func (rs *RateLimiterStore) Delete(ctx context.Context, id string) (*pb.DeleteRateLimiterResponse, error) {
	kv := rs.cli.KV()

	key := fmt.Sprintf("rateLimiters/%s", id)

	_, err := kv.Delete(key, nil)
	if err != nil {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Ratelimiter %s not found.", id))
	}

	// delete from map
	delete(rs.rateLimiters, id)

	return &pb.DeleteRateLimiterResponse{
		Deleted: true,
	}, nil
}

func (rs *RateLimiterStore) IsRequestAllowed(ctx context.Context, id string) (*pb.AllowResponse, error) {
	rateLimiter, err := rs.Get(ctx, id)
	if err != nil {
		log.Printf("%v", err)
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Ratelimiter %s not found.", id))
	}

	rs.createOrUpdateLimiter(rateLimiter, false)

	switch rateLimiter.Type {
	case "tokenBucket":
		limiter, ok := rs.rateLimiters[rateLimiter.Id].(*rate.Limiter)
		if ok {
			log.Printf("broj dostupnih tokena: %f", limiter.Tokens())
			allowed := limiter.Allow()
			log.Printf("%v", allowed)
			return &pb.AllowResponse{
				Allowed: allowed,
			}, nil
		}
		return nil, fmt.Errorf("ne moz da se preuzme limiter iz mape")
	case "leakyBucket":
		limiter := rs.rateLimiters[rateLimiter.Id].(ratelimit.Limiter)
		limiter.Take()
		allowed := true
		return &pb.AllowResponse{
			Allowed: allowed,
		}, nil
	case "slidingWindow":
		limiter := rs.rateLimiters[rateLimiter.Id].(*slidingwindow.Limiter)
		allowed := limiter.Allow()
		return &pb.AllowResponse{
			Allowed: allowed,
		}, nil

	default:
		return nil, fmt.Errorf("algorithm is not implemented")
	}

}

func (rs *RateLimiterStore) createOrUpdateLimiter(rateLimiter *pb.RateLimiter, forUpdate bool) error {
	var limiter interface{}

	if forUpdate && rs.rateLimiters[rateLimiter.Id] == nil {
		return fmt.Errorf("Rate limiter not found")
	}

	// already in map and no need for update
	if !forUpdate && rs.rateLimiters[rateLimiter.Id] != nil {
		log.Println("already in map, skip")
		return nil
	}

	switch rateLimiter.Type {
	case "tokenBucket":
		limiter = rate.NewLimiter(rate.Limit(rateLimiter.ReqPerSec), int(rateLimiter.Burst))
	case "leakyBucket":
		limiter = ratelimit.New(int(rateLimiter.ReqPerSec))
	case "slidingWindow":
		limiter, _ = slidingwindow.NewLimiter(time.Second, rateLimiter.ReqPerSec, func() (slidingwindow.Window, slidingwindow.StopFunc) {
			return slidingwindow.NewLocalWindow()
		})
	}

	rs.rateLimiters[rateLimiter.Id] = limiter

	return nil
}
