package shipped_test

import "testing"

// Each shipped bridge gives as from the sender's lasting id.
func TestBridgesFromIsLasting(t *testing.T) {
	t.Parallel()
	t.Run("telegram", func(t *testing.T) { t.Parallel(); telegramFromIsLasting(t) })
	t.Run("slack", func(t *testing.T) { t.Parallel(); slackFromIsLasting(t) })
	t.Run("gitlab", func(t *testing.T) { t.Parallel(); gitlabFromIsLasting(t) })
	t.Run("jira", func(t *testing.T) { t.Parallel(); jiraFromIsLasting(t) })
}

// Each shipped bridge hands over nothing its own bot or another bot sent.
func TestBridgesBotsNotHeard(t *testing.T) {
	t.Parallel()
	t.Run("telegram", func(t *testing.T) { t.Parallel(); telegramBotsNotHeard(t) })
	t.Run("slack", func(t *testing.T) { t.Parallel(); slackBotsNotHeard(t) })
	t.Run("gitlab", func(t *testing.T) { t.Parallel(); gitlabBotsNotHeard(t) })
	t.Run("jira", func(t *testing.T) { t.Parallel(); jiraBotsNotHeard(t) })
}

// The Slack and Telegram outs take direct, and can send to a person; the
// GitLab and Jira outs do not take it.
func TestBridgesOutTakesDirectWhereItCan(t *testing.T) {
	t.Parallel()
	for name, direct := range map[string]bool{"slack": true, "telegram": true, "gitlab": false, "jira": false} {
		d := shippedDraft(t, "bridges/"+name+"/out.yaml")
		takes, _ := d.Definition.Input["properties"].(map[string]any)
		if _, ok := takes["direct"]; ok != direct || takes["to"] == nil || takes["text"] == nil {
			t.Errorf("%s's out takes %v", name, takes)
		}
	}
}
