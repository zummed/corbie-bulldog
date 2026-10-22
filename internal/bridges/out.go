package bridges

import (
	"context"
	"maps"

	"github.com/zummed/corbie-bulldog/internal/requests"
	"github.com/zummed/corbie/client"
)

// Carry carries one message by a run of the bridge's out.yaml, under the
// name the message gives and with its labels, and with the office's,
// kind=out and bridge. The caller looks for a run of that name first. A
// bridge the office does not have, or one with a problem, carries
// nothing.
func (b *Bridges) Carry(ctx context.Context, m requests.Message) error {
	r := b.Engine.Reading()
	br, d := r.Office.Bridge(m.Bridge), b.definitions(r)[m.Bridge]
	if br == nil || !d.usable() {
		return nil
	}
	labels := maps.Clone(m.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels["bulldog"], labels["kind"], labels["bridge"] = r.Office.ID, "out", m.Bridge
	_, err := b.corbie.Start(ctx, br.Out, client.Start{Run: m.Run, Labels: labels, Input: map[string]any{
		"settings": br.Settings, "to": m.To, "direct": m.Direct, "text": m.Text, "choices": m.Choices,
		"ref": orNull(m.Ref), "answers": orNull(m.Answers)}})
	return err
}

// Direct says whether a bridge reaches a person directly: its out takes
// direct.
func (b *Bridges) Direct(bridge string) bool {
	d := b.definitions(b.Engine.Reading())[bridge]
	return d.usable() && d.direct
}

// orNull gives a text, or null for none.
func orNull(text string) any {
	if text == "" {
		return nil
	}
	return text
}
