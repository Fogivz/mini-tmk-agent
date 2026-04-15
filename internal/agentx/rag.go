package agentx

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type RagSnippet struct {
	Path    string
	Score   int
	Content string
}

type Retriever struct {
	docs []ragDoc
}

type ragDoc struct {
	path    string
	content string
	tokens  map[string]int
}

func NewRetriever(knowledgeDir string) *Retriever {
	r := &Retriever{docs: make([]ragDoc, 0)}
	if strings.TrimSpace(knowledgeDir) == "" {
		return r
	}

	_ = filepath.WalkDir(knowledgeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".txt" && ext != ".md" && ext != ".json" {
			return nil
		}

		bs, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		content := string(bs)
		r.docs = append(r.docs, ragDoc{
			path:    path,
			content: content,
			tokens:  tokenize(content),
		})
		return nil
	})

	return r
}

func (r *Retriever) Retrieve(query string, topK int) []RagSnippet {
	if topK <= 0 {
		topK = 3
	}
	if len(r.docs) == 0 {
		return nil
	}

	q := tokenize(query)
	if len(q) == 0 {
		return nil
	}

	results := make([]RagSnippet, 0, len(r.docs))
	for _, doc := range r.docs {
		score := overlapScore(q, doc.tokens)
		if score == 0 {
			continue
		}
		results = append(results, RagSnippet{
			Path:    doc.path,
			Score:   score,
			Content: clip(doc.content, 1200),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results
}

func overlapScore(a, b map[string]int) int {
	score := 0
	for token, av := range a {
		if bv, ok := b[token]; ok {
			if av < bv {
				score += av
			} else {
				score += bv
			}
		}
	}
	return score
}

func tokenize(s string) map[string]int {
	tokens := make(map[string]int)
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	for _, f := range fields {
		if len(f) < 2 {
			continue
		}
		tokens[f]++
	}
	return tokens
}

func clip(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "\n..."
}
