package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	speech "github.com/google/go-genproto/googleapis/cloud/speech/v1beta1"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

var (
	serviceKey    = flag.String("key", "", "path to service account key created at https://console.developers.google.com/apis/credentials/serviceaccountkey")
	bufSize       = flag.Int("bufSize", 1024, "size in bytes of the read buffer")
	ratePerSecond = flag.Float64("rate", 10, "samples to send per second")
)

func main() {
	flag.Parse()
	log.Println("starting")
	keyContents, err := ioutil.ReadFile(*serviceKey)
	if err != nil {
		log.Fatalln(err)
	}
	if err := runAsync(keyContents); err != nil {
		log.Fatalln(err)
	}
}

func runAsync(keyContents []byte) error {
	ctx := context.Background()
	client, err := speechClient(ctx, keyContents)
	if err != nil {
		return err
	}
	stream, err := client.StreamingRecognize(ctx)
	if err != nil {
		return err
	}

	go func() {
		err = stream.Send(&speech.StreamingRecognizeRequest{
			StreamingRequest: &speech.StreamingRecognizeRequest_StreamingConfig{
				StreamingConfig: &speech.StreamingRecognitionConfig{
					Config: &speech.RecognitionConfig{
						Encoding:   speech.RecognitionConfig_LINEAR16, // TODO(): parameterize
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
		limiter := rate.NewLimiter(rate.Limit(float64(time.Second)/(*ratePerSecond)), 0)
		for rerr == nil {
			fmt.Println("waiting")
			limiter.Wait(ctx)
			_, rerr = os.Stdin.Read(buf)
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
	for {
		resp, err := stream.Recv()
		log.Println("Recv():", err, resp)
		if err != nil {
			return err
		}
	}
	return nil
}

func run(keyContents []byte) error {
	ctx := context.Background()
	client, err := speechClient(ctx, keyContents)
	if err != nil {
		return err
	}

	stdinContent, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	resp, err := client.SyncRecognize(ctx, &speech.SyncRecognizeRequest{
		Config: &speech.RecognitionConfig{
			Encoding:   speech.RecognitionConfig_LINEAR16, // TODO(): parameterize
			SampleRate: 8000,
		},
		Audio: &speech.RecognitionAudio{
			AudioSource: &speech.RecognitionAudio_Content{
				Content: stdinContent,
			},
		},
	})
	if err != nil {
		return err
	}
	for _, result := range resp.Results {
		for _, alt := range result.GetAlternatives() {
			fmt.Println(alt.Transcript)
		}
	}
	return nil
}

func speechClient(ctx context.Context, keyContents []byte) (speech.SpeechClient, error) {
	acct, err := oauth.NewServiceAccountFromKey(
		keyContents,
		"https://www.googleapis.com/auth/cloud-platform",
	)
	if err != nil {
		return nil, err
	}
	conn, err := gax.DialGRPC(ctx,
		gax.WithEndpoint("speech.googleapis.com:443"),
		gax.WithDialOptions(
			grpc.WithPerRPCCredentials(acct),
			grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")),
		),
	)
	return speech.NewSpeechClient(conn), err
}
