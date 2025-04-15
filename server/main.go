package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "file-service/proto/files"
)

type server struct {
	pb.UnimplementedFileServiceServer
	storagePath string
}

func (s *server) Upload(stream pb.FileService_UploadServer) error {
	var file *os.File
	var fileName string
	var fileSize int64

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		if fileName == "" {
			fileName = chunk.FileName
			filePath := filepath.Join(s.storagePath, fileName)
			file, err = os.Create(filePath)
			if err != nil {
				return status.Error(codes.Internal, err.Error())
			}
			defer file.Close()
		}

		n, err := file.Write(chunk.Content)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		fileSize += int64(n)
	}

	return stream.SendAndClose(&pb.UploadStatus{
		Message: fmt.Sprintf("Received %d bytes", fileSize),
		Success: true,
	})
}

func (s *server) Download(req *pb.FileRequest, stream pb.FileService_DownloadServer) error {
	filePath := filepath.Join(s.storagePath, req.FileName)
	file, err := os.Open(filePath)
	if err != nil {
		return status.Error(codes.NotFound, "file not found")
	}
	defer file.Close()

	buf := make([]byte, 64*1024) // 64KB chunks
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}

		err = stream.Send(&pb.Chunk{
			Content:  buf[:n],
			FileName: req.FileName,
		})
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}

func (s *server) GetMetadata(ctx context.Context, req *pb.FileRequest) (*pb.FileMetadata, error) {
	filePath := filepath.Join(s.storagePath, req.FileName)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, status.Error(codes.NotFound, "file not found")
	}

	return &pb.FileMetadata{
		FileName:   req.FileName,
		Size:       uint64(fileInfo.Size()),
		CreatedAt:  fileInfo.ModTime().Format(time.RFC3339),
		ModifiedAt: fileInfo.ModTime().Format(time.RFC3339),
	}, nil
}

func main() {
	storagePath := "./storage"
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Fatalf("failed to create storage dir: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterFileServiceServer(s, &server{storagePath: storagePath})

	log.Println("Server started on :50051")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
