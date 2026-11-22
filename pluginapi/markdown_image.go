package pluginapi

// markdown_image.go —— markdown 正文图片的取数结果（`#18` (b)）。
//
// 与 `AttachmentBlob` 分开一个类型：附件图的身份是**宿主登记的附件**（id/名字/MIME），
// markdown 正文图的身份是**作者写在正文里的目的地**（URL 或路径），两者除了
// "原始字节"之外没有共同字段。共用一个结构就会逼出一堆"这一档填什么"的假字段。

// MarkdownImageBytes 是一张 markdown 正文图片的原始字节。
type MarkdownImageBytes struct {
	// Bytes 是**未解码**的原始字节。解码在插件侧（`internal/imgdecode`），
	// 与消息区图片面同一条链：壳只管取数，格式认不认识由解码器说了算。
	Bytes []byte
	// MIME 是壳探测到的媒体类型（http 响应的 Content-Type；本地文件按扩展名）；
	// 空 = 不确定。它不是判据——真正的判据是能不能解码出位图。
	MIME string
}
