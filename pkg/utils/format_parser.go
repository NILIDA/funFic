package utils

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// ReadBookContent читает содержимое книги в зависимости от формата
func ReadBookContent(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(getFileExtension(filePath))

	switch ext {
	case ".txt":
		return string(content), nil
	case ".md", ".markdown":
		return parseMarkdown(content), nil
	case ".html", ".htm":
		return parseHTML(content), nil
	default:
		// Для неизвестных форматов пытаемся прочитать как текст
		return string(content), nil
	}
}

func getFileExtension(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		return filename[idx:]
	}
	return ""
}

// parseMarkdown конвертирует Markdown в HTML
func parseMarkdown(content []byte) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(content)

	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer))
}

// parseHTML конвертирует HTML в читаемый текст (упрощенная версия)
func parseHTML(content []byte) string {
	htmlString := string(content)

	// Удаляем теги скриптов и стилей
	htmlString = removeTag(htmlString, "script")
	htmlString = removeTag(htmlString, "style")
	htmlString = removeTag(htmlString, "nav")
	htmlString = removeTag(htmlString, "header")
	htmlString = removeTag(htmlString, "footer")

	// Заменяем HTML теги на переносы строк
	htmlString = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(htmlString, "\n")
	htmlString = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(htmlString, "\n")
	htmlString = regexp.MustCompile(`<div[^>]*>`).ReplaceAllString(htmlString, "\n")
	htmlString = regexp.MustCompile(`<h[1-6][^>]*>`).ReplaceAllString(htmlString, "\n")

	// Удаляем все остальные HTML теги
	htmlString = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(htmlString, "")

	// Убираем лишние пробелы и переносы
	htmlString = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(htmlString, "\n\n")
	htmlString = strings.TrimSpace(htmlString)

	return htmlString
}

// removeTag удаляет указанный HTML тег с содержимым
func removeTag(html, tagName string) string {
	pattern := `<` + tagName + `(?s:.*?)</` + tagName + `>`
	return regexp.MustCompile(pattern).ReplaceAllString(html, "")
}

// ExtractFirstLines извлекает первые N строк для предпросмотра
func ExtractFirstLines(content string, lines int) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var result strings.Builder
	count := 0

	for scanner.Scan() && count < lines {
		result.WriteString(scanner.Text() + "\n")
		count++
	}

	return result.String()
}

// IsTextFile проверяет, является ли файл текстовым
func IsTextFile(filename string) bool {
	ext := strings.ToLower(getFileExtension(filename))
	textExtensions := []string{".txt", ".md", ".markdown", ".html", ".htm", ".rtf", ".docx"}

	for _, textExt := range textExtensions {
		if ext == textExt {
			return true
		}
	}
	return false
}

// IsEditableFormat проверяет, можно ли редактировать файл
func IsEditableFormat(filename string) bool {
	ext := strings.ToLower(getFileExtension(filename))
	editableExtensions := []string{".txt", ".md", ".markdown"}

	for _, editableExt := range editableExtensions {
		if ext == editableExt {
			return true
		}
	}
	return false
}

func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
