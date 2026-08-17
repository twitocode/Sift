package indexer

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/twitocode/sift/internal/common"
)

var POSTING_BYTES = int64(binary.Size(common.Posting{}))
var indexDir = "indexdata"

func DumpIndex(stats *common.IndexStats, index map[string][]common.Posting) error {
	info, err := os.Stat(indexDir)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(indexDir, 0755); err != nil {
			return fmt.Errorf("could not create index directory %q: %w", indexDir, err)
		}
	case err != nil:
		return fmt.Errorf("could not inspect index path %q: %w", indexDir, err)
	case !info.IsDir():
		return fmt.Errorf("index path %q exists but is not a directory", indexDir)
	}

	termsFile, err := os.Create(filepath.Join(indexDir, "terms.dat"))

	if err != nil {
		return fmt.Errorf("could not create terms file: %w", err)
	}

	defer termsFile.Close()
	postingsFile, err := os.Create(filepath.Join(indexDir, "postings.dat"))

	if err != nil {
		return fmt.Errorf("could not create postings file: %w", err)
	}

	defer postingsFile.Close()

	termsWriter := bufio.NewWriter(termsFile)
	postingsWriter := bufio.NewWriter(postingsFile)
	// buffer := make([]byte, binary.MaxVarintLen64)

	totalBytes := int64(0)
	//TODO: switch to delta encoding for page id

	for term, postings := range index {
		//each posting is 10 bytes 4 + 4 + 1 + 1

		count := int64(len(postings))
		byteOffset := totalBytes

		if err := binary.Write(postingsWriter, binary.LittleEndian, postings); err != nil {
			return fmt.Errorf("failed to write postings for term %s: %w", term, err)
		}

		totalBytes += count * POSTING_BYTES

		if _, err := fmt.Fprintf(termsWriter, "%s %d %d\n", term, byteOffset, count); err != nil {
			return fmt.Errorf("failed to write term index entry: %w", err)
		}
	}

	if err := termsWriter.Flush(); err != nil {
		return err
	}
	if err := postingsWriter.Flush(); err != nil {
		return err
	}

	return nil
}

func LoadIndex() map[string][]common.Posting {
	out := make(map[string][]common.Posting)

	termsFile, err := os.Open(filepath.Join(indexDir, "terms.dat"))

	if err != nil {
		fmt.Printf("Terms reading could not be started  - io error %v", err)
    return out
	}

	defer termsFile.Close()
	postingsFile, err := os.Open(filepath.Join(indexDir, "postings.dat"))

	if err != nil {
		fmt.Printf("Postings reading could not started - io error %v", err)
    return out
	}

	defer postingsFile.Close()
	termsReader := bufio.NewScanner(termsFile)

	for termsReader.Scan() {
		line := termsReader.Text()

    //try not to use strings.split
		data := strings.Fields(line)

    if len(data) != 3 {
      fmt.Println("Incorrect fields for postings")
      return out
    }
		term := data[0]
		byteOffset, err := strconv.ParseInt(data[1], 10, 64)
		if err != nil {
      fmt.Println("Byte offset could not be parsed")
			return out
		}
		count, err := strconv.ParseInt(data[2], 10, 64)
		if err != nil {
      fmt.Println("Postings count offset could not be parsed")
			return out
		}

		sectionLength := count * POSTING_BYTES
		section := io.NewSectionReader(postingsFile, byteOffset, sectionLength)
		postings := make([]common.Posting, count)

		if err := binary.Read(section, binary.LittleEndian, postings); err != nil {
			return out
		}

		out[term] = postings
	}

	if termsReader.Err() != nil {
		fmt.Printf("Term scanning error %v", termsReader.Err())
		return out
	}

	return out
}
