package main

import (
	"context"

	"github.com/Nordlys-Labs/openai-go"
	"github.com/Nordlys-Labs/openai-go/responses"
)

func main() {
	client := openai.NewClient()
	ctx := context.Background()

	question := "Write me a haiku about computers"

	resp, err := client.Responses.New(ctx, responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(question)},
		Model: openai.ChatModelGPT4,
	})

	if err != nil {
		panic(err)
	}

	println(resp.OutputText())
}
