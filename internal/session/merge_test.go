package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeSlashCommands(t *testing.T) {
	cases := []struct {
		name   string
		blocks []Block
		want   []string // expected BlockType sequence
	}{
		{
			name: "adjacent local_command + command_output merges",
			blocks: []Block{
				&LocalCommand{Name: "focus", Args: "src/", LineNum: 1},
				&System{Subtype: "command_output", Content: "Focused on src/", LineNum: 2},
			},
			want: []string{"slash_merged"},
		},
		{
			name: "unpaired local_command stays standalone",
			blocks: []Block{
				&LocalCommand{Name: "clear", LineNum: 1},
				&AssistantText{Text: "done"},
			},
			want: []string{"local_command", "assistant_text"},
		},
		{
			name: "unpaired command_output stays standalone",
			blocks: []Block{
				&System{Subtype: "command_output", Content: "output", LineNum: 1},
			},
			want: []string{"system"},
		},
		{
			name: "consecutive pairs both merge",
			blocks: []Block{
				&LocalCommand{Name: "focus", LineNum: 1},
				&System{Subtype: "command_output", Content: "out1", LineNum: 2},
				&LocalCommand{Name: "help", LineNum: 3},
				&System{Subtype: "command_output", Content: "out2", LineNum: 4},
			},
			want: []string{"slash_merged", "slash_merged"},
		},
		{
			name: "local_command followed by non-output system stays separate",
			blocks: []Block{
				&LocalCommand{Name: "focus", LineNum: 1},
				&System{Subtype: "away_summary", Content: "away", LineNum: 2},
			},
			want: []string{"local_command", "system"},
		},
		{
			name: "merged block preserves command fields",
			blocks: []Block{
				&LocalCommand{Name: "focus", Args: "src/pkg", LineNum: 10},
				&System{Subtype: "command_output", Content: "Focused!", LineNum: 11},
			},
			want: []string{"slash_merged"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := mergeSlashCommands(c.blocks)
			got := make([]string, len(result))
			for i, b := range result {
				got[i] = b.BlockType()
			}
			assert.Equal(t, c.want, got)

			// For the "preserves command fields" case, verify content
			if c.name == "merged block preserves command fields" {
				sm := result[0].(*SlashMerged)
				assert.Equal(t, "focus", sm.Command.Name)
				assert.Equal(t, "src/pkg", sm.Command.Args)
				assert.Equal(t, "Focused!", sm.Output)
				assert.Equal(t, 10, sm.LineNum)
			}
		})
	}
}
