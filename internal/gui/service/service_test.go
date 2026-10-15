package service

import "testing"

// Notifier が仕様どおり可変長引数のシグネチャであることをコンパイル時に検証する。
type recordingNotifier struct{ got []Topic }

func (r *recordingNotifier) Notify(topics ...Topic) { r.got = append(r.got, topics...) }

var (
	_ Notifier        = (*recordingNotifier)(nil)
	_ LifecycleSource = lifecycleFunc(nil)
)

type lifecycleFunc func() LifecycleState

func (f lifecycleFunc) State() LifecycleState { return f() }

func TestNotifier_Variadic(t *testing.T) {
	var n recordingNotifier
	var notifier Notifier = &n

	notifier.Notify(TopicHosts, TopicStatus)
	notifier.Notify(TopicAll)

	want := []Topic{TopicHosts, TopicStatus, TopicAll}
	if len(n.got) != len(want) {
		t.Fatalf("got %v, want %v", n.got, want)
	}
	for i := range want {
		if n.got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, n.got[i], want[i])
		}
	}
}

func TestTopicValues(t *testing.T) {
	// トピック名は JS 側の htmx トリガー名（mp:<topic>）と一致する必要がある。
	tests := map[Topic]string{
		TopicHosts:      "hosts",
		TopicForwards:   "forwards",
		TopicStatus:     "status",
		TopicHeader:     "header",
		TopicCredential: "credential",
		TopicToast:      "toast",
		TopicSettings:   "settings",
		TopicAll:        "all",
	}
	for topic, want := range tests {
		if string(topic) != want {
			t.Errorf("Topic %q, want %q", topic, want)
		}
	}
}

func TestLifecycleStateValues(t *testing.T) {
	tests := map[LifecycleState]string{
		LifecycleStarting: "starting",
		LifecycleReady:    "ready",
		LifecycleFailed:   "failed",
	}
	for state, want := range tests {
		if string(state) != want {
			t.Errorf("LifecycleState %q, want %q", state, want)
		}
	}
}
