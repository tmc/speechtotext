package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/api/option"

	"cloud.google.com/go/speech/apiv1beta1"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1beta1"
)

var (
	serviceKey    = flag.String("key", "", "path to service account key created at https://console.developers.google.com/apis/credentials/serviceaccountkey")
	bufSize       = flag.Int("bufSize", 10240, "size in bytes of the read buffer")
	ratePerSecond = flag.Duration("rate", 1*time.Millisecond, "rate at which to send buffers")
	verbose       = flag.Bool("v", false, "if true show response details")
	languageCode  = flag.String("lang", "", "the language of the supplied audio as a BCP-47 language tag, e.g: fa-IR See: https://cloud.google.com/speech/docs/languages ")
)

func main() {
	flag.Parse()
	if err := runAsync(*serviceKey); err != nil {
		log.Fatalln(err)
	}
}

func runAsync(credsPath string) error {
	ctx := context.Background()
	client, err := speech.NewClient(ctx, option.WithServiceAccountFile(credsPath))
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
		err = stream.Send(&speechpb.StreamingRecognizeRequest{
			StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
				StreamingConfig: &speechpb.StreamingRecognitionConfig{
					Config: &speechpb.RecognitionConfig{
						LanguageCode: *languageCode,                         //"fa-IR" "en-US"
						Encoding:     speechpb.RecognitionConfig_LINEAR16, // TODO(): paramaterize
						SampleRate:   16000,
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
			err := stream.Send(&speechpb.StreamingRecognizeRequest{
				StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
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
		if resp.EndpointerType == speechpb.StreamingRecognizeResponse_END_OF_SPEECH {
			out.CloseWithError(io.EOF)
		}
	}
}
