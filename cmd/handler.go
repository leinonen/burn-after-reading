package cmd

import (
	"bytes"
	"context"
	_ "embed" // for embedding HTML
	"encoding/base64"
	"encoding/json"
	"html/template"
	"log"
	"net/url"
	"strings"

	"burn.leinonen.ninja/internal/crypto"
	"burn.leinonen.ninja/internal/db"
	"github.com/aws/aws-lambda-go/events"
)

//go:embed templates/index.html
var indexHtmlFile string

var dbClient *db.DBClient // used for Lambda Function URL

// SetDBClient sets the dbClient for Lambda Function URL usage
func SetDBClient(client *db.DBClient) {
	dbClient = client
}

func Router(ctx context.Context, rawEvent json.RawMessage) (interface{}, error) {
	var urlReq events.LambdaFunctionURLRequest
	if err := json.Unmarshal(rawEvent, &urlReq); err == nil && urlReq.RequestContext.HTTP.Method != "" {
		log.Printf("[URL] method=%s path=%s body=%s", urlReq.RequestContext.HTTP.Method, urlReq.RequestContext.HTTP.Path, urlReq.Body)
		switch {
		case urlReq.RequestContext.HTTP.Method == "GET" && urlReq.RequestContext.HTTP.Path == "/":
			resp := serveCreateFormURL()
			return resp, nil
		case urlReq.RequestContext.HTTP.Method == "POST" && urlReq.RequestContext.HTTP.Path == "/create":
			resp, err := handleCreateSnippetURL(ctx, urlReq)
			return resp, err
		case urlReq.RequestContext.HTTP.Method == "GET" && strings.HasPrefix(urlReq.RequestContext.HTTP.Path, "/snippet/"):
			id := strings.TrimPrefix(urlReq.RequestContext.HTTP.Path, "/snippet/")
			resp := serveRevealFormURL(id)
			return resp, nil
		case urlReq.RequestContext.HTTP.Method == "POST" && urlReq.RequestContext.HTTP.Path == "/reveal":
			resp, err := handleRevealSnippetURL(ctx, urlReq)
			return resp, err
		default:
			log.Printf("[URL] 404 Not Found: method=%s path=%s", urlReq.RequestContext.HTTP.Method, urlReq.RequestContext.HTTP.Path)
			return events.LambdaFunctionURLResponse{StatusCode: 404, Body: "404 Not Found"}, nil
		}
	}

	log.Printf("Unknown event format or missing method/path")
	return events.LambdaFunctionURLResponse{StatusCode: 400, Body: "Bad Request"}, nil
}

func renderTemplate(name string, data any) string {
	var buf bytes.Buffer
	tmpl := template.Must(template.New("index").Parse(indexHtmlFile))
	err := tmpl.ExecuteTemplate(&buf, "index", map[string]any{"Content": template.HTML(renderSubTemplate(tmpl, name, data))})
	if err != nil {
		return "<p>Template error</p>"
	}
	return buf.String()
}

func renderSubTemplate(tmpl *template.Template, name string, data any) string {
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "<p>Subtemplate error</p>"
	}
	return buf.String()
}

func serveCreateFormURL() events.LambdaFunctionURLResponse {
	html := renderTemplate("create_form", nil)
	return htmlResponseURL(html)
}

func serveRevealFormURL(id string) events.LambdaFunctionURLResponse {
	html := renderTemplate("reveal_form", map[string]any{"ID": id})
	return htmlResponseURL(html)
}

func handleCreateSnippetURL(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	var body string
	if req.IsBase64Encoded {
		decoded, err := decodeBase64(req.Body)
		if err != nil {
			log.Printf("ERROR: failed to decode base64 body in handleCreateSnippetURL: %v", err)
			return errorResponseURL("Internal server error (body decode)"), nil
		}
		body = decoded
	} else {
		body = req.Body
	}
	data, _ := url.ParseQuery(body)
	message := data.Get("message")
	password := data.Get("password")
	log.Printf("handleCreateSnippetURL: message length=%d, body=%q", len(message), body)

	if dbClient == nil {
		log.Printf("ERROR: dbClient is nil in handleCreateSnippetURL")
		return errorResponseURL("Internal server error (db not initialized)"), nil
	}

	encrypted, salt, err := crypto.Encrypt([]byte(message), []byte(password))
	if err != nil {
		log.Printf("Encryption error: %v", err)
		return errorResponseURL("Unable to encrypt message"), nil
	}

	id, err := dbClient.SaveSnippet(ctx, encrypted, salt)
	if err != nil {
		log.Printf("SaveSnippet error: %v", err)
		return errorResponseURL("Unable to save message"), nil
	}

	// Build full URL
	proto := "https://"
	host := req.Headers["x-forwarded-host"]
	if host == "" {
		host = req.Headers["host"]
	}
	if host == "" {
		host = "localhost"
		proto = "http://"
	}
	fullURL := proto + host + "/snippet/" + id

	html := renderTemplate("create_success", map[string]any{"FullURL": fullURL})
	return htmlResponseURL(html), nil
}

func handleRevealSnippetURL(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	log.Printf("handleRevealSnippetURL: headers=%v, isBase64Encoded=%v, body=%q", req.Headers, req.IsBase64Encoded, req.Body)
	var body string
	if req.IsBase64Encoded {
		decoded, err := decodeBase64(req.Body)
		if err != nil {
			log.Printf("ERROR: failed to decode base64 body: %v", err)
			return errorResponseURL("Internal server error (body decode)"), nil
		}
		body = decoded
	} else {
		body = req.Body
	}
	data, _ := url.ParseQuery(body)
	id := data.Get("id")
	password := data.Get("password")
	log.Printf("handleRevealSnippetURL: id=%q, body=%q", id, body)

	if id == "" {
		log.Printf("ERROR: id is empty in handleRevealSnippetURL. Full body: %q", body)
		return errorResponseURL("Missing snippet ID"), nil
	}

	if dbClient == nil {
		log.Printf("ERROR: dbClient is nil in handleRevealSnippetURL")
		return errorResponseURL("Internal server error (db not initialized)"), nil
	}

	encrypted, salt, err := dbClient.GetAndDeleteSnippet(ctx, id)
	if err != nil {
		log.Printf("GetAndDeleteSnippet error: %v", err)
		return errorResponseURL("Message not found"), nil
	}

	plaintext, err := crypto.Decrypt(encrypted, []byte(password), salt)
	if err != nil {
		log.Printf("Decrypt error: %v", err)
		return errorResponseURL("Wrong password"), nil
	}

	html := renderTemplate("reveal_success", map[string]any{"Message": string(plaintext)})
	return htmlResponseURL(html), nil
}

func decodeBase64(s string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func htmlResponseURL(body string) events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/html"},
		Body:       body,
	}
}

func errorResponseURL(msg string) events.LambdaFunctionURLResponse {
	html := renderTemplate("error", map[string]any{"Error": msg})
	return htmlResponseURL(html)
}
