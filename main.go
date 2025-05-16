package main

import (
	"context"
	"log"

	"burn.leinonen.ninja/cmd"
	"burn.leinonen.ninja/internal/db"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Debug: DescribeTable to verify connectivity and permissions
	desc, err := dynamoClient.DescribeTable(context.Background(), &dynamodb.DescribeTableInput{
		TableName: aws.String("snippets"),
	})
	if err != nil {
		log.Printf("DescribeTable error: %v", err)
	} else {
		log.Printf("DescribeTable success: Table status is %s", string(desc.Table.TableStatus))
	}

	dbInstance := &db.DBClient{
		Client:    dynamoClient,
		TableName: "snippets",
	}

	cmd.SetDBClient(dbInstance) // for Lambda Function URL
	lambda.Start(cmd.Router)
}
