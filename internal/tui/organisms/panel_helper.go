package organisms

// PanelInnerSize はパネルの外枠サイズから内部描画領域のサイズを計算する。
// ボーダー分（幅: 左右各2, 高さ: 上下各1）を差し引き、最小値でクランプする。
func PanelInnerSize(width, height int) (innerWidth, innerHeight int) {
	innerWidth = max(width-4, 10)
	innerHeight = max(height-2, 1)
	return
}
