package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mr-destructive/burrow/plugins"
	"github.com/mr-destructive/burrow/plugins/db/libsqlssg"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	lambda.Start(handler)
}

func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	if req.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: "",
		}, nil
	}
	ctx := context.Background()
	dbName := os.Getenv("TURSO_DATABASE_NAME")
	dbToken := os.Getenv("TURSO_DATABASE_AUTH_TOKEN")

	var err error
	dbString := fmt.Sprintf("libsql://%s?authToken=%s", dbName, dbToken)
	db, err := sql.Open("libsql", dbString)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	defer db.Close()

	queries := libsqlssg.New(db)
	if _, err := db.ExecContext(ctx, plugins.DDL); err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}

	var payload plugins.Payload
	log.Printf("Headers: %v", req.Headers)
	log.Printf("hx-request??? %v", req.Headers["hx-request"])
	if req.Headers["hx-request"] == "true" {
		formData, err := url.ParseQuery(req.Body)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "Invalid form Payload"), nil
		}
		metadata := make(map[string]interface{})
		err = json.Unmarshal([]byte(formData.Get("metadata")), &metadata)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "Invalid metadata Payload"), nil
		}
		payload = plugins.Payload{
			Username: formData.Get("username"),
			Password: formData.Get("password"),
			Title:    formData.Get("title"),
			Post:     formData.Get("content"),
			Metadata: metadata,
		}
	} else {
		err = json.Unmarshal([]byte(req.Body), &payload)
		if err != nil {
			return errorResponse(http.StatusInternalServerError, "Invalid Payload"), nil
		}
	}
	log.Printf("Payload: %v", payload)
	user, err := queries.GetUser(ctx, payload.Username)
	log.Printf("User: %v", user)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "User Not Found"), nil
	}
	if !Authenticate(payload.Username, user.Password, payload.Password) {
		return errorResponse(http.StatusInternalServerError, "Authentication Failed"), nil
	}

	post, err := plugins.CreatePostPayload(payload, int(user.ID), user.Name)
	log.Printf("Post: %v", post)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	_, err = queries.CreatePost(ctx, post)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}

	if req.Headers["hx-request"] == "true" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type":                 "text/html",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `<div class="success-message">Post created successfully!</div>`,
		}, nil
	}

	return jsonResponse(http.StatusOK, post), nil
}

func Authenticate(username, hashedPassword, rawPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(rawPassword))
	fmt.Println(err)
	if err != nil {
		fmt.Println("Authentication Failure")
		return false
	}
	return true
}
func jsonResponse(statusCode int, data interface{}) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(data)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, Authorization",
		},
		Body: string(body),
	}
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return jsonResponse(statusCode, map[string]string{"error": message})
}
