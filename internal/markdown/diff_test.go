package markdown

import "testing"

func TestExtractBlocks(t *testing.T) {
	src := "# Title\n\nFirst paragraph.\n\nSecond paragraph.\n"
	blocks := ExtractBlocks(src)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	for i, b := range blocks {
		if b.Anchor == "" {
			t.Errorf("block %d missing anchor", i)
		}
		if b.HTML == "" {
			t.Errorf("block %d missing html", i)
		}
	}
}

func TestDiffPureAdd(t *testing.T) {
	oldB := ExtractBlocks("# Title\n")
	newB := ExtractBlocks("# Title\n\nAdded paragraph.\n")
	hunks := DiffBlocks(oldB, newB)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[0].Op != OpKeep {
		t.Errorf("expected first hunk keep, got %s", hunks[0].Op)
	}
	if hunks[1].Op != OpAdd {
		t.Errorf("expected second hunk add, got %s", hunks[1].Op)
	}
}

func TestDiffPureDelete(t *testing.T) {
	oldB := ExtractBlocks("# Title\n\nGone.\n")
	newB := ExtractBlocks("# Title\n")
	hunks := DiffBlocks(oldB, newB)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	if hunks[1].Op != OpDel {
		t.Errorf("expected second hunk del, got %s", hunks[1].Op)
	}
}

func TestDiffReplaceParagraph(t *testing.T) {
	oldB := ExtractBlocks("# Title\n\nThe quick brown fox jumps over the lazy dog every morning.\n")
	newB := ExtractBlocks("# Title\n\nThe quick brown fox leaps over the lazy dog each morning.\n")
	hunks := DiffBlocks(oldB, newB)
	var found bool
	for _, h := range hunks {
		if h.Op == OpReplace {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a replace hunk, got %+v", hunks)
	}
}

func TestDiffFirstVersion(t *testing.T) {
	newB := ExtractBlocks("# Hello\n\nWorld.\n")
	hunks := DiffBlocks(nil, newB)
	if len(hunks) != 2 {
		t.Fatalf("expected 2 hunks, got %d", len(hunks))
	}
	for _, h := range hunks {
		if h.Op != OpAdd {
			t.Errorf("expected add, got %s", h.Op)
		}
	}
}
