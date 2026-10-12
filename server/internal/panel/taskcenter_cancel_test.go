package panel

import (
	"testing"
	"time"
)

// TestQueueCancelFlag lifecycle：取消标记的读写与复位。
func TestQueueCancelFlag(t *testing.T) {
	p := &Panel{}
	q := p.queue()
	if q.isCancelled() {
		t.Fatal("新队列不应处于取消态")
	}
	q.mu.Lock()
	q.running = true
	q.cancel = true
	q.mu.Unlock()
	if !q.isCancelled() {
		t.Fatal("置位后应读到取消态")
	}
	q.mu.Lock()
	q.cancel = false
	q.mu.Unlock()
	if q.isCancelled() {
		t.Fatal("复位后不应处于取消态")
	}
}

// TestRunQueueItemsCancelSkipsPending 协作式取消：置取消标记后 runQueueItems
// 应把剩余 pending 项统一标 skipped、running 复位、cancel 标记复位。
// accts 为空（没有执行 goroutine 在跑）→ 收尾逻辑立即生效。
func TestRunQueueItemsCancelSkipsPending(t *testing.T) {
	p := &Panel{}
	q := p.queue()
	q.mu.Lock()
	q.running = true
	q.startedAt = time.Now()
	q.cancel = true // 模拟「正在跑时收到取消」
	q.items = []queueItem{
		{UID: "u1", Kind: "growth", Code: "a", Status: "done"},
		{UID: "u1", Kind: "growth", Code: "b", Status: "pending"},
		{UID: "u2", Kind: "growth", Code: "c", Status: "pending"},
	}
	q.mu.Unlock()

	p.runQueueItems(nil, q.items, 1)

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.running {
		t.Fatal("收尾后 running 应复位为 false")
	}
	if q.cancel {
		t.Fatal("收尾后取消标记应复位为 false（下一轮干净启动）")
	}
	for _, it := range q.items {
		if it.Code == "b" || it.Code == "c" {
			if it.Status != "skipped" || it.Message != "已取消" {
				t.Fatalf("pending 项 %s 应被标 skipped/已取消，实际 %s/%s", it.Code, it.Status, it.Message)
			}
		}
		if it.Code == "a" && it.Status != "done" {
			t.Fatalf("已完成项不应被改写，实际 %s", it.Status)
		}
	}
}
