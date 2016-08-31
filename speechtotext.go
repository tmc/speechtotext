package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
	speech "google.golang.org/genproto/googleapis/cloud/speech/v1beta1"
)

var (
	serviceKey    = flag.String("key", "", "path to service account key created at https://console.developers.google.com/apis/credentials/serviceaccountkey")
	bufSize       = flag.Int("bufSize", 10240, "size in bytes of the read buffer")
	ratePerSecond = flag.Duration("rate", 1*time.Millisecond, "rate at which to send buffers")
	verbose       = flag.Bool("v", false, "if true show response details")
)

func main() {
	flag.Parse()
	if err := runAsync(*serviceKey); err != nil {
		log.Fatalln(err)
	}
}

func runAsync(credsPath string) error {
	ctx := context.Background()
	client, err := speechClient(ctx, credsPath)
	if err != nil {
		return err
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		return err
	}
	in, out := io.Pipe()
	go func() {
		io.Copy(out, os.Stdin)
		out.CloseWithError(io.EOF)
	}()

	go func() {
		err = stream.Send(&speech.StreamingRecognizeRequest{
			StreamingRequest: &speech.StreamingRecognizeRequest_StreamingConfig{
				StreamingConfig: &speech.StreamingRecognitionConfig{
					Config: &speech.RecognitionConfig{
						Encoding:   speech.RecognitionConfig_LINEAR16, // TODO(): paramaterize
						SampleRate: 16000,
					},
					InterimResults: true,
				},
			},
		})
		var (
			rerr error
		)
		buf := make([]byte, *bufSize)
		limiter := rate.NewLimiter(rate.Every(*ratePerSecond), 1)
		for rerr == nil {
			limiter.Wait(ctx)
			_, rerr = io.ReadAtLeast(in, buf, *bufSize)
			err := stream.Send(&speech.StreamingRecognizeRequest{
				StreamingRequest: &speech.StreamingRecognizeRequest_AudioContent{
					AudioContent: buf,
				},
			})
			if err != nil {
				rerr = err
			}
		}
		if err := stream.CloseSend(); err != nil {
			log.Println("issue closing stream:", err)
		}
	}()
	maxLine := 0
	defer fmt.Println()
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if *verbose {
			fmt.Println(resp)
		}
		if resp.Results != nil {
			fmt.Printf("\r%s\r", strings.Repeat(" ", maxLine))
			if resp.Results[0].Stability > 0.0 && resp.Results[0].Stability < 0.6 {
				fmt.Printf("?")
			}
			transcript := resp.Results[0].Alternatives[0].Transcript
			if len(transcript)+1 > maxLine {
				maxLine = len(transcript) + 1
			}
			fmt.Print(transcript)
			if *verbose {
				fmt.Println()
			}
		}
		if resp.EndpointerType == speech.StreamingRecognizeResponse_END_OF_SPEECH {
			out.CloseWithError(io.EOF)
		}
	}
}

func speechClient(ctx context.Context, credsPath string) (speech.SpeechClient, error) {
	creds, err := oauth.NewServiceAccountFromFile(credsPath, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial("speech.googleapis.com:443",
		grpc.WithPerRPCCredentials(creds),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")),
	)
	return speech.NewSpeechClient(conn), err
}
