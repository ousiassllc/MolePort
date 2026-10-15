package shell

import (
	"reflect"
	"testing"

	"github.com/ousiassllc/moleport/internal/gui/service"
)

func newRecordingEmitter() (*VisibilityNotifier, *[][]string) {
	var emitted [][]string
	n := NewVisibilityNotifier(func(topics []string) { emitted = append(emitted, topics) })
	return n, &emitted
}

func TestVisibilityNotifier_HiddenDropsNotifications(t *testing.T) {
	n, emitted := newRecordingEmitter()

	n.Notify(service.TopicHosts, service.TopicStatus)

	if len(*emitted) != 0 {
		t.Errorf("非表示中に emit された: %v", *emitted)
	}
}

func TestVisibilityNotifier_VisibleEmitsTopicNames(t *testing.T) {
	n, emitted := newRecordingEmitter()
	n.SetVisible(true)
	*emitted = nil // 表示遷移時の all を除外

	n.Notify(service.TopicHosts, service.TopicStatus)
	n.Notify()

	want := [][]string{{"hosts", "status"}}
	if !reflect.DeepEqual(*emitted, want) {
		t.Errorf("emitted = %v, want %v", *emitted, want)
	}
}

func TestVisibilityNotifier_ShowResyncsOnce(t *testing.T) {
	n, emitted := newRecordingEmitter()

	n.SetVisible(true)
	n.SetVisible(true) // 既に表示中なら再送しない
	n.SetVisible(false)
	n.Notify(service.TopicForwards)
	n.SetVisible(true)

	want := [][]string{{"all"}, {"all"}}
	if !reflect.DeepEqual(*emitted, want) {
		t.Errorf("emitted = %v, want %v", *emitted, want)
	}
	if !n.Visible() {
		t.Error("Visible() = false, want true")
	}
}
