package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/4sp1/must"
)

var (
	mustHaveFile   = must.Have(must.ExitController[*os.File](1))
	mustHaveInt64  = must.Have(must.ExitController[int64](1))
	mustHaveInt    = must.Have(must.ExitController[int](1))
	mustHaveString = must.Have(must.ExitController[string](1))
	mustHandle     = must.HandleError(must.ExitHandler(1))
	mustDo         = must.Handle(must.ExitHandler(1))
)

func main() {
	keyFile := flag.String("key", "key.txt", "groq api key file")
	promptFile := flag.String("prompt", "user.txt", "groq prompt file")
	responseFile := flag.String("response", "response.txt", "response destination file")
	model := flag.String("model", "openai/gpt-oss-120b", "groq model")
	config := flag.Bool("config", false, "prepare required files and exit")
	stdin := flag.Bool("stdin", false, "read prompt from stdin instead of prompt file")
	stdout := flag.Bool("stdout", false, "print outpout to stdout instead of destination file")
	flag.Parse()

	if *config {
		{
			f := mustHaveFile(os.Create(*promptFile))
			mustDo(f.Close)
			fmt.Println("edit", *promptFile, "to update prompt")
		}
		mustDo(func() error {
			_, err := os.Stat(*keyFile)
			if err == nil {
				f := mustHaveFile(os.Open(*keyFile))
				var b bytes.Buffer
				mustHaveInt64(io.Copy(&b, f))
				if len(strings.TrimSpace(b.String())) == 0 {
					fmt.Println("you need to copy you groq key in", *keyFile)
				}
			}
			if errors.Is(err, os.ErrNotExist) {
				f := mustHaveFile(os.OpenFile(*keyFile, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600))
				mustDo(f.Close)
				fmt.Println("copy groq api key in", *keyFile)
				return nil
			}
			return err
		})
		return
	}

	key := mustHaveString(func() (string, error) {
		f, err := os.Open(*keyFile)
		if err != nil {
			return "", fmt.Errorf("open key file: %w", err)
		}
		defer mustDo(f.Close)
		var b bytes.Buffer
		if _, err := io.Copy(&b, f); err != nil {
			return "", fmt.Errorf("copy key file: %w", err)
		}
		return strings.TrimSpace(b.String()), nil
	}())

	var prompt string
	if *stdin {
		var b bytes.Buffer
		mustHaveInt64(io.Copy(&b, os.Stdin))
		prompt = strings.TrimSpace(b.String())
	} else {
		prompt = mustHaveString(func() (string, error) {
			f := mustHaveFile(os.Open(*promptFile))
			defer mustDo(f.Close)
			var b bytes.Buffer
			mustHaveInt64(io.Copy(&b, f))
			return strings.TrimSpace(b.String()), nil
		}())
	}

	var r r
	{
		var out bytes.Buffer

		{
			f := mustHaveFile(os.Create("response.json"))
			defer mustDo(f.Close)

			post(io.MultiWriter(&out, f), *model, prompt, key)
		}

		mustHandle(json.NewDecoder(&out).Decode(&r))
	}

	var out io.Writer
	if *stdout {
		out = os.Stdout
	} else {
		f := mustHaveFile(os.Create(*responseFile))
		defer mustDo(f.Close)
		out = f
	}
	mustHaveInt(out.Write([]byte(r.Choices[0].Message.Content)))
}

func post(w io.Writer, model, prompt, key string) {
	f := mustHaveFile(os.Create("payload.json"))
	defer mustDo(f.Close)

	mustHandle(
		json.NewEncoder(f).Encode(
			j{
				Model: model,
				Messages: []msg{
					{
						Role:    "user",
						Content: prompt,
					},
				},
			},
		),
	)

	cmd := exec.Command("curl", "https://api.groq.com/openai/v1/chat/completions", "-s",
		"-H", "Content-Type: application/json",
		"-H", "Authorization: Bearer "+key,
		"-d", "@payload.json")

	cmd.Stdout = w

	mustDo(cmd.Run)
}

type j struct {
	Model    string `json:"model"`
	Messages []msg  `json:"messages"`
}

type msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type r struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message message `json:"message"`
}

type message struct {
	Content string `json:"content"`
}
