(WIP) TFT Calculator for Trait Optimization and Maximization


A backend engine built in Go that calculates optimal team combinations for Teamfight Tactics (TFT).


Technical Stack

Language: Go

Infrastructure: AWS Lambda & Amazon S3

IaC: Terraform

CI/CD: GitHub Actions



How It Works

Data Storage: Game data (champions and traits) is stored as CSV files in an Amazon S3 bucket.

Serverless Compute: An AWS Lambda function pulls the data and runs a recursive backtracking algorithm to find unit combinations that meet a specific "active trait" goal.

Optimization: The Go binary uses global state caching to ensure that data ingestion from S3 only happens once per Lambda container, keeping execution times under 50ms.

Deployment: Terraform manages the entire AWS lifecycle, from IAM permissions to S3 bucket policies and Lambda triggers.



Project Structure

main.go: The core solver logic and Lambda handler.

main.tf: Terraform configuration for AWS infrastructure.

.github/workflows: Automated CI/CD pipeline.

data/: Sample game datasets used for the solver.
