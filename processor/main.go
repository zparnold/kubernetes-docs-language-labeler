package main

import (
	"fmt"
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/events"
	"encoding/json"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"os"
	"regexp"
	"strings"
	"github.com/kataras/golog"
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
		if evt == "opened"{
			golog.Info("Calculating delta of files changed")
			files := getChangedFiles(*webhookPayloadData.Number)
			label := calculateLanguage(&files)
			applyLanguageLabels(*webhookPayloadData.Number, label)
		}
	}
}

func getChangedFiles(prNum int) []*github.CommitFile {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GH_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	files, _, err := client.PullRequests.ListFiles(ctx, "zparnold", "website", prNum, &github.ListOptions{})
	if err != nil{
		golog.Fatal(err)
	}
	golog.Info("There are ",len(files)," files changed")
	return files
}

func calculateLanguage(files *[]*github.CommitFile) string {
	baseUrl := "https://github.com/zparnold/website/blob/"
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
		golog.Info("After hacking, this file has path: ", pathUrl)
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
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: "4b531b237884b2f93ff25d834bbc0fe018e6de68"},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	_, _, err := client.Issues.AddLabelsToIssue(ctx, "zparnold", "website", prNum, []string{label})
	if err != nil{
		os.Exit(1)
	}
}
