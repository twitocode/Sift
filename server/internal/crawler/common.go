package crawler

import (
	"fmt"
	"os"
	"strings"
)

func SaveResultsToFile(links []URL) error {
	if len(links) == 0 {
		return nil
	}

	strLinks := make([]string, len(links))
	for i, link := range links {
		strLinks[i] = link.String()
	}
	content := strings.Join(strLinks, "\n") + "\n"

	err := os.WriteFile("result.txt", []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write results to file: %w", err)
	}

	fmt.Println("Wrote results to file")
	return nil
}
