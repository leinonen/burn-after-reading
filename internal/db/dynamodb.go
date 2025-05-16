package db

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type DBClient struct {
	Client    *dynamodb.Client
	TableName string
}

func (d *DBClient) SaveSnippet(ctx context.Context, encrypted []byte, salt []byte) (string, error) {
	id := uuid.NewString()
	ttl := time.Now().Add(24 * time.Hour).Unix()

	_, err := d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.TableName),
		Item: map[string]types.AttributeValue{
			"ID":            &types.AttributeValueMemberS{Value: id},
			"EncryptedData": &types.AttributeValueMemberB{Value: encrypted},
			"Salt":          &types.AttributeValueMemberB{Value: salt},
			"TTL":           &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
		},
	})
	return id, err
}

func (d *DBClient) GetAndDeleteSnippet(ctx context.Context, id string) ([]byte, []byte, error) {
	out, err := d.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(d.TableName),
		Key:       map[string]types.AttributeValue{"ID": &types.AttributeValueMemberS{Value: id}},
	})
	if err != nil || out.Item == nil {
		return nil, nil, fmt.Errorf("snippet not found")
	}

	encrypted := out.Item["EncryptedData"].(*types.AttributeValueMemberB).Value
	salt := out.Item["Salt"].(*types.AttributeValueMemberB).Value

	_, _ = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.TableName),
		Key:       map[string]types.AttributeValue{"ID": &types.AttributeValueMemberS{Value: id}},
	})
	return encrypted, salt, nil
}
