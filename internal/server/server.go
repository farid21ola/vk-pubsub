package server

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"vk-pubsub/internal/config"
	"vk-pubsub/internal/subpub"
	"vk-pubsub/pkg/proto"
)

type Server struct {
	proto.UnimplementedPubSubServer
	grpcServer *grpc.Server
	pubSub     subpub.SubPub
	logger     *log.Logger
	cfg        *config.Config
}

func New(cfg *config.Config, log *log.Logger, pubSub subpub.SubPub) *Server {
	grpcServer := grpc.NewServer()

	server := &Server{
		grpcServer: grpcServer,
		pubSub:     pubSub,
		logger:     log,
		cfg:        cfg,
	}

	proto.RegisterPubSubServer(grpcServer, server)
	return server
}

func (s *Server) Start() error {
	s.logger.Printf("gRPC сервер запущен на %s", s.cfg.GRPCAddress())
	listener, err := net.Listen("tcp", s.cfg.GRPCAddress())
	if err != nil {
		s.logger.Printf("Ошибка при создании слушателя: %v", err)
		return err
	}

	return s.grpcServer.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Printf("Останавливаем gRPC сервер...")

	s.logger.Printf("Закрываем систему публикации-подписки...")
	err := s.pubSub.Close(ctx)
	if err != nil {
		s.logger.Printf("Ошибка при закрытии pubsub: %v", err)
	}

	stopped := make(chan struct{})
	go func() {
		s.grpcServer.Stop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		s.logger.Printf("Таймаут остановки сервера")
	case <-stopped:
		s.logger.Printf("Сервер успешно остановлен")
	}

	s.logger.Printf("Сервер полностью остановлен")
	return nil
}

func (s *Server) Subscribe(req *proto.SubscribeRequest, stream proto.PubSub_SubscribeServer) error {
	s.logger.Printf("Получен запрос на подписку для ключа: %s", req.Key)

	if req.Key == "" {
		s.logger.Printf("Получен пустой ключ в запросе на подписку")
		return status.Errorf(codes.InvalidArgument, "ключ подписки не может быть пустым")
	}

	eventCh := make(chan string, 100)

	ctx := stream.Context()

	subscribe, err := s.pubSub.Subscribe(req.Key, func(msg interface{}) {
		if data, ok := msg.(string); ok {
			select {
			case eventCh <- data:
			case <-ctx.Done():
			default:
				s.logger.Printf("Буфер сообщений для подписчика ключа %s заполнен, сообщение пропущено", req.Key)
			}
		} else {
			s.logger.Printf("Получено сообщение неверного типа: %T", msg)
		}
	})
	defer subscribe.Unsubscribe()

	if err != nil {
		s.logger.Printf("Ошибка создания подписки: %v", err)
		return status.Errorf(codes.Internal, "ошибка создания подписки: %v", err)
	}

	s.logger.Printf("Подписка создана для ключа: %s", req.Key)

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("Клиент отключился от подписки для ключа: %s", req.Key)
			return nil
		case data, ok := <-eventCh:
			if !ok {
				s.logger.Printf("Канал событий закрыт для ключа: %s", req.Key)
				return nil
			}
			event := &proto.Event{
				Data: data,
			}
			if err := stream.Send(event); err != nil {
				s.logger.Printf("Ошибка отправки события: %v", err)
				return status.Errorf(codes.Internal, "ошибка отправки события: %v", err)
			}
			s.logger.Printf("Отправлено событие для ключа %s: %s", req.Key, data)
		}
	}
}

func (s *Server) Publish(ctx context.Context, req *proto.PublishRequest) (*emptypb.Empty, error) {
	s.logger.Printf("Получен запрос на публикацию для ключа: %s", req.Key)

	if req.Key == "" {
		s.logger.Printf("Получен пустой ключ в запросе на публикацию")
		return nil, status.Errorf(codes.InvalidArgument, "ключ публикации не может быть пустым")
	}

	err := s.pubSub.Publish(req.Key, req.Data)
	if err != nil {
		s.logger.Printf("Ошибка публикации: %v", err)
		return nil, status.Errorf(codes.Internal, "ошибка публикации: %v", err)
	}

	s.logger.Printf("Опубликовано событие для ключа %s: %s", req.Key, req.Data)
	return &emptypb.Empty{}, nil
}
