package app

import (
	"sort"
	"strings"
)

func FilterCandidates(candidates []RepoCandidate, query string) []RepoCandidate {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return append([]RepoCandidate(nil), candidates...)
	}

	needle := strings.ToLower(trimmedQuery)
	filtered := make([]RepoCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		displayName := candidateDisplayName(candidate)
		if displayName == "" {
			continue
		}
		if candidateMatchesQuery(displayName, needle) {
			filtered = append(filtered, candidate)
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		leftName := candidateDisplayName(filtered[i])
		rightName := candidateDisplayName(filtered[j])

		leftLen := len([]rune(leftName))
		rightLen := len([]rune(rightName))
		if leftLen != rightLen {
			return leftLen < rightLen
		}

		leftLower := strings.ToLower(leftName)
		rightLower := strings.ToLower(rightName)
		if leftLower != rightLower {
			return leftLower < rightLower
		}
		return leftName < rightName
	})

	return filtered
}

func candidateDisplayName(candidate RepoCandidate) string {
	name := strings.TrimSpace(candidate.Name)
	if name != "" {
		return name
	}
	return strings.TrimSpace(candidate.Path)
}

func candidateMatchesQuery(displayName, query string) bool {
	if query == "" {
		return true
	}

	haystack := []rune(strings.ToLower(displayName))
	needle := []rune(strings.ToLower(query))
	if len(needle) == 0 {
		return true
	}

	needleIndex := 0
	for _, r := range haystack {
		if r != needle[needleIndex] {
			continue
		}
		needleIndex++
		if needleIndex == len(needle) {
			return true
		}
	}
	return false
}
