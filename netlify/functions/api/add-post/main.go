package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/mr-destructive/ssg/plugins"
	"github.com/mr-destructive/ssg/plugins/db/libsqlssg"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	lambda.Start(handler)
}

func handler(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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
	err = json.Unmarshal([]byte(req.Body), &payload)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	user, err := queries.GetUser(ctx, payload.Username)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	if !Authenticate(payload.Username, payload.Password, user.Password) {
		return errorResponse(http.StatusInternalServerError, "Authentication Failed"), nil
	}

	post, err := plugins.CreatePostPayload(payload, int(user.ID), user.Name)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	_, err = queries.CreatePost(ctx, post)
	if err != nil {
		return errorResponse(http.StatusInternalServerError, "Database connection failed"), nil
	}
	return jsonResponse(http.StatusOK, map[string]string{}), nil

}

func Authenticate(username, rawPassword, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(rawPassword), []byte(hashedPassword))
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
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}

func errorResponse(statusCode int, message string) events.APIGatewayProxyResponse {
	return jsonResponse(statusCode, map[string]string{"error": message})
}
