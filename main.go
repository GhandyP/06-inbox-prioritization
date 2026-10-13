package main

import (
	"context"
	"log"
	"os"

	"github.com/GhandyP/adk-go-enterprise-cookbook/internal/adkhelper"
)

func main() {
	if err := adkhelper.Run(
		context.Background(),
		os.Args[1:],
		"case_06_inbox_prioritization",
		"A text-only starting point for prioritize the inbox.",
		"Prioritize inbox messages supplied as text using urgent, today, later, or ignore. Explain evidence and do not invent due dates.",
	); err != nil {
		log.Fatal(err)
	}
}
