# Kubernetes Docs Language PR Labeler
Note: I don't have a cool Greek Nautical title for this yet, suggestions are welcome.

## Purpose
The purpose of the code in this repo is to parse open pull requests against
the [https://github.com/kubernetes/website](https://github.com/kubernetes/website) repo, and
assign a label to the PR of which language it believes is responsible for the PR.

## Working with this code
This repo is based off of the Serverless framework and has its implementation in AWS Lambda. To work with this code,
you'll need the prerequesites found here: [https://serverless.com/blog/framework-example-golang-lambda-support/](https://serverless.com/blog/framework-example-golang-lambda-support/)

## Roadmap
With the initial implementation working, the next steps are to:

1) Build a Travis CI CI/CD pipeline so this tool can be updated without the need for @zparnold
1) Potentially integrate its functionality into Tide from the Kubernetes Test-Infra group