package ssh

import (
	"crypto/rand"
	"math/big"
	"time"
)

// defaultInitialDelay は initial_delay が 0 以下（不正値）のときに使う再接続の初期遅延。
// 0 のままだと max_retries: 0（無制限）で待機なしの再接続ループになる。
const defaultInitialDelay = time.Second

// initialBackoff は再接続の最初の素の遅延を返す。
// initial が 0 以下の場合は既定値に補正し、maxDelay が正のときは maxDelay でキャップする。
func initialBackoff(initial, maxDelay time.Duration) time.Duration {
	if initial <= 0 {
		initial = defaultInitialDelay
	}
	if maxDelay > 0 && initial > maxDelay {
		return maxDelay
	}
	return initial
}

// nextBackoff は素の遅延（ジッター加算前）を倍化し、maxDelay でキャップした値を返す。
// maxDelay が 0 以下の場合は上限なしとして扱う。
func nextBackoff(base, maxDelay time.Duration) time.Duration {
	next := base * 2
	if maxDelay > 0 && next > maxDelay {
		return maxDelay
	}
	return next
}

// withJitter は遅延 base に 0-10% のジッターを加えた待機時間を返す。
// 加算後の値は maxDelay を超えない（maxDelay が 0 以下の場合は上限なし）。
func withJitter(base, maxDelay time.Duration) time.Duration {
	wait := base
	// 0-10% のジッターを crypto/rand で生成
	if maxJitter := int64(float64(base) * 0.1); maxJitter > 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(maxJitter)); err == nil {
			wait += time.Duration(n.Int64())
		}
	}
	if maxDelay > 0 && wait > maxDelay {
		return maxDelay
	}
	return wait
}
