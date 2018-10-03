package main

import (
	"fmt"
	"os"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"strings"
	"crypto/hmac"
	"github.com/aws/aws-sdk-go/service/kms"
	"encoding/base64"
	"crypto/sha1"
	"github.com/kataras/golog"
	"encoding/hex"
)

func Handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	svc := sqs.New(sess)
	//Decrypt secret key used in GH HMAC https://developer.github.com/webhooks/#delivery-headers
	kmsSvc := kms.New(sess)
	blob, err := base64.StdEncoding.DecodeString(os.Getenv("GH_SECRET"))
	result, err := kmsSvc.Decrypt(&kms.DecryptInput{CiphertextBlob: blob})
	if err != nil {
		golog.Fatal("Got error decrypting data: ", err)
	}

	//Use github's HMAC to verify the payload before accepting
	if !verifyPayload(request.Headers, request.Body, result.Plaintext){
		golog.Error("Payload didn't pass verification. Exiting...")
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}
	golog.Info("All checks pass, pushing into queue for processing")
	// URL to our queue
	qURL := os.Getenv("SQS_QUEUE_URL")
	_, qErr := svc.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(request.Body)),
		QueueUrl:    &qURL,
	})

	if qErr != nil {
		fmt.Println(qErr.Error())
		return events.APIGatewayProxyResponse{StatusCode: 200}, err
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func main() {
	lambda.Start(Handler)
}

// https://developer.github.com/webhooks/#payloads
func verifyPayload(headers map[string]string, body string, key []byte) bool {
	checks := make(map[string]bool)
	checks["User-Agent"] = checkUserAgent(headers["User-Agent"])
	checks["GH-Event"] = checkEvent(headers["X-GitHub-Event"])
	checks["Signature"] = checkSignature(headers["X-Hub-Signature"], body, key)
	for checkName,check := range checks {
		//if the check fails immediately return a fail
		if !check {
			golog.Error("Failed check: ", checkName)
			return false
		}
	}
	//if it passes all checks, all of them are true
	return true
}

func checkSignature(receivedSignature string, body string, key []byte) bool {
	//trim out the beginning section of the header leaving just the signature
	trimmedHeader := strings.Replace(receivedSignature, "sha1=", "", 1)
	//compute HMAC
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(body))
	computedSignature := hex.EncodeToString(mac.Sum(nil))
	return trimmedHeader == computedSignature
}

func checkUserAgent(receivedUserAgent string) bool {
	golog.Info("Checking User-Agent Header: ", receivedUserAgent)
	return strings.HasPrefix(receivedUserAgent, "GitHub-Hookshot/")
}

func checkEvent(receivedEvent string) bool {
	golog.Info("Checking event type for pull_request. Got event: ", receivedEvent)
	return receivedEvent == "pull_request"
}