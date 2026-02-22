package pathalias

import (
	"testing"
)

func BenchmarkResolveImports(b *testing.B) {
	resolver := makeResolver(
		"/project/src",
		"/project/dist",
		"/project/src",
		map[string][]string{
			"@app/*":  {"src/*"},
			"@lib/*":  {"src/lib/*"},
			"@config": {"src/config"},
		},
	)

	input := `import { UserService } from "@app/services/user";
import { Logger } from "@lib/logger";
import { config } from "@config";
import express from "express";
import { Router } from "./router";`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.ResolveImports(input, "/project/dist/controllers/user.controller.js")
	}
}
