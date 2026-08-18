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
	"golang.org/x/exp/mmap"
)

var POSTING_BYTES = int64(binary.Size(common.Posting{}))
var indexDir = "index_data"

func DumpIndex(stats *common.IndexStats, index map[string][]common.Posting) error {
  //TODO: atomic writing with .tmp file
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

func LoadTerms() map[string]TermData {
	out := make(map[string]TermData)

	termsFile, err := os.Open(filepath.Join(indexDir, "terms.dat"))
	if err != nil {
		fmt.Printf("Terms reading could not be started  - io error %v", err)
		return out
	}
	defer termsFile.Close()

	termsReader := bufio.NewScanner(termsFile)
	for termsReader.Scan() {
		data := strings.Fields(termsReader.Text())
		if len(data) != 3 {
			fmt.Println("Incorrect fields for postings")
			return out
		}

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

		out[data[0]] = TermData{Count: count, ByteOffset: byteOffset}
	}

	if termsReader.Err() != nil {
		fmt.Printf("Term scanning error %v", termsReader.Err())
	}

	return out
}

func LoadIndex() map[string][]common.Posting {
	out := make(map[string][]common.Posting)
	terms := LoadTerms()
	if len(terms) == 0 {
		return out
	}

	postingsFile, err := os.Open(filepath.Join(indexDir, "postings.dat"))
	if err != nil {
		fmt.Printf("Postings reading could not started - io error %v", err)
		return out
	}
	defer postingsFile.Close()

	for term, data := range terms {
		sectionLength := data.Count * POSTING_BYTES
		section := io.NewSectionReader(postingsFile, data.ByteOffset, sectionLength)
		postings := make([]common.Posting, data.Count)

		if err := binary.Read(section, binary.LittleEndian, postings); err != nil {
			return out
		}

		out[term] = postings
	}

	return out
}

func CreateMMapReader() *mmap.ReaderAt {
	reader, err := mmap.Open(filepath.Join(indexDir, "postings.dat"))

	if err != nil {
		fmt.Printf("failed to open file: %v", err)
		return nil
	}

	return reader
}

func LoadIndexSection(reader *mmap.ReaderAt, byteOffset int64, count int64) []common.Posting {

	//CANNOT DECODE INTO A 0 LENGTH SLICE MAKE SURE TO ADD count VARIABLE STUPID
	postings := make([]common.Posting, count)
	sectionLength := count * POSTING_BYTES

	buf := make([]byte, sectionLength)
	bytes, err := reader.ReadAt(buf, byteOffset)

	if err != nil {
		fmt.Printf("failed to read data: %v", err)
		return postings
	}

	if int64(bytes) != sectionLength {
		fmt.Printf("incorrect amount of bytes read: received - %d, expected %d", bytes, sectionLength)
		return []common.Posting{}
	}
	if bytes, err := binary.Decode(buf, binary.LittleEndian, &postings); err != nil {
		fmt.Printf("Postings could not be loaded, tried to load %d bytes: %v", bytes, err)
		return postings
	}

	return postings
}
