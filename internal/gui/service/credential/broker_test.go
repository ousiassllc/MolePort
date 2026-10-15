package credential

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/gui/service"
)

// recordNotifier は Notify されたトピックを記録する。
type recordNotifier struct {
	mu     sync.Mutex
	topics []service.Topic
}

func (n *recordNotifier) Notify(topics ...service.Topic) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.topics = append(n.topics, topics...)
}

func (n *recordNotifier) all() []service.Topic {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]service.Topic(nil), n.topics...)
}

type countWindow struct {
	mu    sync.Mutex
	shows int
}

func (w *countWindow) Show() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shows++
}

type result struct {
	resp core.CredentialResponse
	err  error
}

// ask は Callback を別ゴルーチンで呼び、結果をチャネルで返す。
func ask(b *Broker, req core.CredentialRequest) <-chan result {
	ch := make(chan result, 1)
	go func() {
		resp, err := b.Callback()(req)
		ch <- result{resp, err}
	}()
	return ch
}

func TestBroker_ResolvePassword(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n, win := &recordNotifier{}, &countWindow{}
		b := NewBroker(n, win)
		got := ask(b, core.CredentialRequest{Type: core.CredentialPassword, Host: "prod"})
		synctest.Wait()

		v, ok := b.Pending()
		if !ok || v.Host != "prod" || v.Type != string(core.CredentialPassword) || v.Remaining != 0 {
			t.Fatalf("Pending() = %+v, %v", v, ok)
		}
		value := []byte("s3cret")
		if err := b.Resolve(v.RequestID, Input{Value: value}); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		r := <-got
		if r.err != nil || r.resp.Value != "s3cret" {
			t.Fatalf("Callback = %+v, %v", r.resp, r.err)
		}
		if string(value) != "\x00\x00\x00\x00\x00\x00" {
			t.Errorf("Resolve 後のバッファ = %q, want ゼロ化", value)
		}
		if _, ok := b.Pending(); ok {
			t.Error("解決後も待機中の要求が残っている")
		}
		if win.shows != 1 {
			t.Errorf("ウィンドウ前面化 = %d 回, want 1", win.shows)
		}
		// 通知はトピック名のみ（値を含めない）。
		want := []service.Topic{service.TopicCredential, service.TopicCredential}
		if got := n.all(); !reflect.DeepEqual(got, want) {
			t.Errorf("Notify = %v, want %v", got, want)
		}
	})
}

func TestBroker_KeyboardInteractiveAlignsAnswers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBroker(&recordNotifier{}, nil)
		got := ask(b, core.CredentialRequest{
			Type:    core.CredentialKeyboardInteractive,
			Host:    "prod",
			Prompts: []core.PromptInfo{{Prompt: "Code: ", Echo: true}, {Prompt: "PIN: ", Echo: false}},
		})
		synctest.Wait()

		v, _ := b.Pending()
		if len(v.Prompts) != 2 || v.Prompts[0].Label != "Code: " || !v.Prompts[0].Echo || v.Prompts[1].Echo {
			t.Fatalf("Prompts = %+v", v.Prompts)
		}
		// 回答が不足しても Prompts 数に揃える（不足は空）。
		answer := []byte("123456")
		if err := b.Resolve(v.RequestID, Input{Answers: [][]byte{answer}}); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		r := <-got
		if r.err != nil || !reflect.DeepEqual(r.resp.Answers, []string{"123456", ""}) {
			t.Fatalf("Answers = %q, %v", r.resp.Answers, r.err)
		}
		if string(answer) != "\x00\x00\x00\x00\x00\x00" {
			t.Errorf("回答のバッファがゼロ化されていない: %q", answer)
		}
	})
}

func TestBroker_Cancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBroker(&recordNotifier{}, nil)
		got := ask(b, core.CredentialRequest{Type: core.CredentialPassphrase})
		synctest.Wait()

		v, _ := b.Pending()
		if err := b.Cancel(v.RequestID); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		if r := <-got; !errors.Is(r.err, core.ErrCredentialCancelled) {
			t.Errorf("err = %v, want ErrCredentialCancelled", r.err)
		}
		if err := b.Cancel(v.RequestID); !errors.Is(err, ErrUnknownRequest) {
			t.Errorf("2 回目の Cancel = %v, want ErrUnknownRequest", err)
		}
	})
}

func TestBroker_Timeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := &recordNotifier{}
		b := NewBroker(n, nil)
		start := time.Now()
		got := ask(b, core.CredentialRequest{Type: core.CredentialPassword})
		synctest.Wait()
		v, _ := b.Pending()

		r := <-got
		if !errors.Is(r.err, core.ErrCredentialTimeout) {
			t.Fatalf("err = %v, want ErrCredentialTimeout", r.err)
		}
		if elapsed := time.Since(start); elapsed != core.CredentialTimeout {
			t.Errorf("タイムアウトまで %v, want %v", elapsed, core.CredentialTimeout)
		}
		if _, ok := b.Pending(); ok {
			t.Error("タイムアウト後も待機中の要求が残っている")
		}
		// タイムアウト後の Resolve は期限切れ。
		value := []byte("late")
		if err := b.Resolve(v.RequestID, Input{Value: value}); !errors.Is(err, ErrUnknownRequest) {
			t.Errorf("Resolve = %v, want ErrUnknownRequest", err)
		}
		if string(value) != "\x00\x00\x00\x00" {
			t.Errorf("失敗した Resolve でもバッファをゼロ化する: %q", value)
		}
		// 解除時にもダイアログを閉じるための credential トピックを通知する。
		if got := n.all(); len(got) != 2 || got[1] != service.TopicCredential {
			t.Errorf("Notify = %v", got)
		}
	})
}

func TestBroker_FIFOOneAtATime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewBroker(&recordNotifier{}, nil)
		first := ask(b, core.CredentialRequest{Type: core.CredentialPassword, Host: "a"})
		synctest.Wait()
		second := ask(b, core.CredentialRequest{Type: core.CredentialPassword, Host: "b"})
		synctest.Wait()

		v1, _ := b.Pending()
		if v1.Host != "a" || v1.Remaining != 1 {
			t.Fatalf("先頭 = %+v, want host a・残り 1", v1)
		}
		if err := b.Resolve(v1.RequestID, Input{Value: []byte("pa")}); err != nil {
			t.Fatal(err)
		}
		v2, _ := b.Pending()
		if v2.Host != "b" || v2.Remaining != 0 {
			t.Fatalf("2 件目 = %+v, want host b・残り 0", v2)
		}
		if err := b.Resolve(v2.RequestID, Input{Value: []byte("pb")}); err != nil {
			t.Fatal(err)
		}
		if r := <-first; r.resp.Value != "pa" {
			t.Errorf("1 件目の値 = %q", r.resp.Value)
		}
		if r := <-second; r.resp.Value != "pb" {
			t.Errorf("2 件目の値 = %q", r.resp.Value)
		}
	})
}

func TestBroker_ResolveUnknownID(t *testing.T) {
	b := NewBroker(&recordNotifier{}, nil)
	if err := b.Resolve("gcr-404", Input{}); !errors.Is(err, ErrUnknownRequest) {
		t.Errorf("Resolve = %v, want ErrUnknownRequest", err)
	}
}
