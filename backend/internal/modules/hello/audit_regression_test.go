package hello

// Phase-1.5 对抗式审计（2026-06-11）B-1 回归测试：
// Stop 传入已取消的 ctx 时，若 worker 实际已干净退出，必须返回 nil
// （修复前 select 双臂就绪的随机选取可能把成功停止误报为失败）。

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/plugin/plugintest"
)

func TestStopWithPreCancelledContextAfterWorkerExit(t *testing.T) {
	m := new(Module)
	require.NoError(t, m.Provision(plugintest.NewHost(t)))
	require.NoError(t, m.Validate())
	require.NoError(t, m.Start(context.Background()))

	// 让 worker 先行退出并确认退出完成（直接驱动内部 cancel，规避计时竞态）。
	m.cancel()
	select {
	case <-m.done:
	case <-time.After(5 * time.Second):
		t.Fatal("worker 未在期限内退出")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, m.Stop(cancelled), "worker 已退出时，即便 ctx 已取消 Stop 也应返回 nil")
}
