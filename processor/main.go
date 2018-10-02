package main

import (
	"fmt"
	"context"
	"os"
	"regexp"
	"strings"
	"encoding/json"

	"golang.org/x/oauth2"
	"github.com/google/go-github/github"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/events"
	"github.com/kataras/golog"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
	"encoding/base64"
)

func main() {
	lambda.Start(ReceieveMessage)
}

func ReceieveMessage(ctx context.Context, sqsEvent events.SQSEvent){
	for _,value := range sqsEvent.Records {
		var webhookPayloadData github.PullRequestEvent
		err := json.Unmarshal([]byte(value.Body), &webhookPayloadData)
		if err != nil {
			panic(err)
		}
		evt := webhookPayloadData.GetAction()
		//we only want to label newly opened PRs
		if evt == "opened"{
			golog.Info("Calculating delta of files changed")
			files := getChangedFiles(*webhookPayloadData.Number)
			label := calculateLanguage(&files)
			applyLanguageLabels(*webhookPayloadData.Number, label)
		}
	}
}

func getChangedFiles(prNum int) []*github.CommitFile {
	client, ctx := makeGithubClient()
	files, _, err := client.PullRequests.ListFiles(*ctx, os.Getenv("GH_ORG"), os.Getenv("GH_REPO"), prNum, &github.ListOptions{})
	if err != nil{
		golog.Fatal(err)
	}
	golog.Info("There are ",len(files)," files changed")
	return files
}

func calculateLanguage(files *[]*github.CommitFile) string {
	baseUrl := fmt.Sprintf("https://github.com/%s/%s/blob/", os.Getenv("GH_ORG"), os.Getenv("GH_REPO"))
	re := regexp.MustCompile("\\b[0-9a-f]{5,40}\\b")
	labelPrefix := "language/"
	langKeyRef := make(map[string]int)
	//to ensure english will never evaluate to nothing, we'll initialize it here
	//this is needed later to ensure that the default tiebreaker will be english
	langKeyRef["en"] = 0

	for _,v := range *files{
		golog.Info("Processing file: ", *v.BlobURL)
		match := re.FindStringSubmatch(*v.BlobURL)
		basePath := fmt.Sprintf("%s%s/",baseUrl, match[0])
		blobURL := *v.BlobURL
		pathUrl := strings.Replace(blobURL, basePath, "", 1)
		//we care about enumerating values underneath the content directory
		if strings.HasPrefix(pathUrl, "content") {
			golog.Info("This file lives in a content/ directory")
			ccUrl := strings.Replace(pathUrl, "content/", "", 1)
			twoCharCountryCode := ccUrl[:2]
			langKeyRef[twoCharCountryCode] += 1
		} else {
			//if it isn't underneath the content/ directory count it as english to weight PR's in the direction of
			//someone who could approve a config change for example
			golog.Info("This file appears to be not in a content/ directory")
			langKeyRef["en"] += 1
		}
	}

	//Start with a random key as the max and iterate through map to determine the max
	highestLang := getAnyKey(&langKeyRef)
	for k, v := range langKeyRef{
		if v > langKeyRef[highestLang]{
			highestLang = k
		}
	}
	golog.Info("I believe the highestLang is: ", highestLang)
	golog.Info("Here is the whole map: ", langKeyRef)

	//default to english
	if langKeyRef[highestLang] == langKeyRef["en"]{
		highestLang = "en"
	}

	label := fmt.Sprintf("%s%s",labelPrefix, highestLang)
	return label
}

func getAnyKey(m *map[string]int) string{
	for k := range *m {
		return k
	}
	return ""
}

func applyLanguageLabels(prNum int, label string){
	golog.Info("Now applying this label to this PR: ", prNum, label)
	client, ctx := makeGithubClient()
	_, _, err := client.Issues.AddLabelsToIssue(*ctx, os.Getenv("GH_ORG"), os.Getenv("GH_REPO"), prNum, []string{label})
	if err != nil{
		os.Exit(1)
	}
}

func makeGithubClient() (*github.Client, *context.Context){
	//Make a pseudo-factory method for producing a client and a context in which GH can execute
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2")},
	))
	kmsClient := kms.New(sess)
	blob, err := base64.StdEncoding.DecodeString(os.Getenv("GH_TOKEN"))
	result, err := kmsClient.Decrypt(&kms.DecryptInput{CiphertextBlob: blob})
	if err != nil {
		golog.Fatal("Got error decrypting data: ", err)
	}
	blobString := string(result.Plaintext)
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: blobString},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return client, &ctx
}