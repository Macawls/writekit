package markdown

import "strings"

type HunkOp string

const (
	OpKeep    HunkOp = "keep"
	OpAdd     HunkOp = "add"
	OpDel     HunkOp = "del"
	OpReplace HunkOp = "replace"
)

type Hunk struct {
	Op      HunkOp `json:"op"`
	Anchor  string `json:"anchor"`
	HTML    string `json:"html,omitempty"`
	OldHTML string `json:"oldHtml,omitempty"`
}

const replaceSimilarityThreshold = 0.6

func DiffBlocks(oldBlocks, newBlocks []Block) []Hunk {
	m, n := len(oldBlocks), len(newBlocks)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldBlocks[i-1].Hash == newBlocks[j-1].Hash {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var raw []Hunk
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && oldBlocks[i-1].Hash == newBlocks[j-1].Hash:
			raw = append([]Hunk{{Op: OpKeep, Anchor: newBlocks[j-1].Anchor, HTML: newBlocks[j-1].HTML}}, raw...)
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			raw = append([]Hunk{{Op: OpAdd, Anchor: newBlocks[j-1].Anchor, HTML: newBlocks[j-1].HTML}}, raw...)
			j--
		default:
			raw = append([]Hunk{{Op: OpDel, Anchor: oldBlocks[i-1].Anchor, OldHTML: oldBlocks[i-1].HTML}}, raw...)
			i--
		}
	}

	return collapseReplacements(raw, oldBlocks, newBlocks)
}

func collapseReplacements(hunks []Hunk, oldBlocks, newBlocks []Block) []Hunk {
	oldByAnchor := indexByAnchor(oldBlocks)
	newByAnchor := indexByAnchor(newBlocks)

	out := make([]Hunk, 0, len(hunks))
	for k := 0; k < len(hunks); k++ {
		h := hunks[k]
		if k+1 < len(hunks) {
			next := hunks[k+1]
			if (h.Op == OpDel && next.Op == OpAdd) || (h.Op == OpAdd && next.Op == OpDel) {
				var oldH, newH Hunk
				if h.Op == OpDel {
					oldH, newH = h, next
				} else {
					oldH, newH = next, h
				}
				oldBlock, okOld := oldByAnchor[oldH.Anchor]
				newBlock, okNew := newByAnchor[newH.Anchor]
				if okOld && okNew && oldBlock.Kind == newBlock.Kind && tokenSimilarity(oldBlock.Text, newBlock.Text) >= replaceSimilarityThreshold {
					out = append(out, Hunk{
						Op:      OpReplace,
						Anchor:  newBlock.Anchor,
						HTML:    newBlock.HTML,
						OldHTML: oldBlock.HTML,
					})
					k++
					continue
				}
			}
		}
		out = append(out, h)
	}
	return out
}

func indexByAnchor(blocks []Block) map[string]Block {
	m := make(map[string]Block, len(blocks))
	for _, b := range blocks {
		m[b.Anchor] = b
	}
	return m
}

func tokenSimilarity(a, b string) float64 {
	at := tokenSet(a)
	bt := tokenSet(b)
	if len(at) == 0 && len(bt) == 0 {
		return 1
	}
	inter := 0
	for tok := range at {
		if _, ok := bt[tok]; ok {
			inter++
		}
	}
	union := len(at) + len(bt) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSet(s string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, tok := range strings.Fields(strings.ToLower(s)) {
		set[tok] = struct{}{}
	}
	return set
}
