package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "file-service-system/proto"
)

func uploadFile(client pb.FileServiceClient, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("could not open file: %v", err)
	}
	defer file.Close()

	stream, err := client.Upload(context.Background())
	if err != nil {
		log.Fatalf("could not upload: %v", err)
	}

	buf := make([]byte, 64*1024) // 64KB chunks
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("error reading file: %v", err)
		}

		err = stream.Send(&pb.Chunk{
			Content:  buf[:n],
			FileName: filepath.Base(filename),
		})
		if err != nil {
			log.Fatalf("error sending chunk: %v", err)
		}
	}

	status, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("error receiving status: %v", err)
	}
	fmt.Printf("Upload Status: %v\n", status.Message)
}

func downloadFile(client pb.FileServiceClient, filename string) {
	stream, err := client.Download(context.Background(), &pb.FileRequest{FileName: filename})
	if err != nil {
		log.Fatalf("could not download: %v", err)
	}

	file, err := os.Create(filepath.Join("downloads", filename))
	if err != nil {
		log.Fatalf("could not create file: %v", err)
	}
	defer file.Close()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("error receiving chunk: %v", err)
		}

		_, err = file.Write(chunk.Content)
		if err != nil {
			log.Fatalf("error writing chunk: %v", err)
		}
	}
	fmt.Println("Download completed")
}

func getMetadata(client pb.FileServiceClient, filename string) {
	metadata, err := client.GetMetadata(context.Background(), &pb.FileRequest{FileName: filename})
	if err != nil {
		log.Fatalf("could not get metadata: %v", err)
	}

	fmt.Printf("Metadata for %s:\n", metadata.FileName)
	fmt.Printf("Size: %d bytes\n", metadata.Size)
	fmt.Printf("Created: %s\n", metadata.CreatedAt)
	fmt.Printf("Modified: %s\n", metadata.ModifiedAt)
}

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	// Create downloads directory
	if err := os.MkdirAll("downloads", 0755); err != nil {
		log.Fatalf("failed to create downloads dir: %v", err)
	}

	command := os.Args[1]
    filename := os.Args[2]

    switch command {
    case "upload":
        uploadFile(client, filename)
    case "download":
        downloadFile(client, filename)
    case "metadata":
        getMetadata(client, filename)
    default:
        log.Fatalf("Unknown command: %s", command)
    }
}