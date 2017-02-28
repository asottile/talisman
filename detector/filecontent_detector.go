package detector

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/thoughtworks/talisman/git_repo"
	"strings"
	"math"
)

const BASE64_CHARS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="
const MIN_SECRET_LENGTH = 20
const BASE64_ENTROPY_THRESHOLD = 4.5

type FileContentDetector struct {
	base64Map map[string]bool
	aggressiveDetector *AggressiveFileContentDetector
}

func NewFileContentDetector() *FileContentDetector {
	fc := FileContentDetector{}
	fc.initBase64Map()
	fc.aggressiveDetector = nil
	return &fc
}

func (fc *FileContentDetector) AggressiveMode() *FileContentDetector {
	fc.aggressiveDetector = &AggressiveFileContentDetector{}
	return fc
}

func (fc *FileContentDetector) initBase64Map() {
	fc.base64Map = map[string]bool{}
	for i := 0; i < len(BASE64_CHARS); i++ {
		fc.base64Map[string(BASE64_CHARS[i])] = true
	}
}

func (fc *FileContentDetector) Test(additions []git_repo.Addition, ignores Ignores, result *DetectionResults) {
	for _, addition := range additions {
		if ignores.Deny(addition) {
			log.WithFields(log.Fields{
				"filePath": addition.Path,
			}).Info("Ignoring addition as it was specified to be ignored.")
			result.Ignore(addition.Path, fmt.Sprintf("%s was ignored by .talismanignore", addition.Path))
			continue
		}
		base64Text := fc.checkBase64EncodingForFile(addition.Data)
		if base64Text != "" {
			log.WithFields(log.Fields{
				"filePath": addition.Path,
			}).Info("Failing file as it contains a base64 encoded text.")
			result.Fail(addition.Path, fmt.Sprintf("Expected file to not to contain base64 encoded texts such as: %s", base64Text))
		}
	}
}

func (fc *FileContentDetector) getShannonEntropy(str string, superSet string) float64 {
	if str == "" {
		return 0
	}
	entropy := 0.0
	for _, c := range superSet {
		p := float64(strings.Count(str, string(c))) / float64(len(str))
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func (fc *FileContentDetector) checkBase64Encoding(word string) string {
	entropyCandidates := fc.getEntropyCandidatesWithinWord(word, MIN_SECRET_LENGTH, fc.base64Map)
	for _, candidate := range entropyCandidates {
		entropy := fc.getShannonEntropy(candidate, BASE64_CHARS)
		if entropy > BASE64_ENTROPY_THRESHOLD {
			return word
		}
	}
	if fc.aggressiveDetector != nil {
		return fc.aggressiveDetector.Test(word)
	}
	return ""
}

func (fc *FileContentDetector) getEntropyCandidatesWithinWord(word string, minCandidateLength int, superSet map[string]bool) []string {
	candidates := []string{}
	count := 0
	subSet := ""
	if len(word) < minCandidateLength {
		return candidates
	}
	for _, c := range word {
		char := string(c)
		if superSet[char] {
			subSet += char
			count++
		} else {
			if count > minCandidateLength {
				candidates = append(candidates, subSet)
			}
			subSet = ""
			count = 0
		}
	}
	if count > minCandidateLength {
		candidates = append(candidates, subSet)
	}
	return candidates
}

func (fc *FileContentDetector) checkBase64EncodingForFile(data []byte) string {
	content := string(data)
	return fc.checkEachLine(content)
}

func (fc *FileContentDetector) checkEachLine(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		res := fc.checkEachWord(line)
		if res != "" {
			return res
		}
	}
	return ""
}

func (fc *FileContentDetector) checkEachWord(line string) string {
	words := strings.Fields(line)
	for _, word := range words {
		res := fc.checkBase64Encoding(word)
		if res != "" {
			return res
		}
	}
	return ""
}