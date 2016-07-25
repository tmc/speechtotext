package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	speech "github.com/google/go-genproto/googleapis/cloud/speech/v1beta1"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
)

var serviceKey = flag.String("key", "", "path to service account key created at https://console.developers.google.com/apis/credentials/serviceaccountkey")

func main() {
	flag.Parse()
	keyContents, err := ioutil.ReadFile(*serviceKey)
	if err != nil {
		log.Fatalln(err)
	}
	if err := run(keyContents); err != nil {
		log.Fatalln(err)
	}
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
			Encoding:   speech.RecognitionConfig_LINEAR16, // TODO(): paramaterize
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
