// Command vsinfo 生成 Windows 的 VERSIONINFO 资源（`.syso`），交给 Go 链接器嵌进 exe。
//
// 为什么自己写而不是装 windres/rsrc：本机没有 windres；`rc.exe` 只出 `.res`，
// 转 COFF 还要 `cvtres`（这台机器上没有）；`rsrc` 那类第三方生成器只能做图标和
// manifest、做不了 VERSIONINFO。而这一项的全部价值就是**任务管理器的「描述」列
// 不是空白**——进程列表里一眼看出"这是 DSH GUI GO"。为它引一条 C 工具链或一个
// 只在打包机上存在的第三方生成器，是把构建变复杂：换来的是别人克隆仓库后
// `go build ./...` 直接多一个前置条件。
//
// 做法与 `../DSH-GO/cmd/dshtui/internal/vsinfo/main.go` 一致（那份已在真机上验过：
// 资源管理器/任务管理器能读出 ProductName 与描述）。照搬比另发明一个稳。
//
// **名字不写死在这个程序里**：从 `cmd/app/brand.go` 读 `productName` / `binaryName` /
// `version` 三个声明（它们就是产品名与版本号的唯一出处）。资源里的名字和产物里的
// 名字对不上，是这类资源最常见的坏法——两边各自写一份，改一处忘一处。
//
// 用法（`go generate ./cmd/app` 会调；手工重生成也用同一条）：
//
//	cd cmd/app && go run ./internal/vsinfo -o versioninfo_windows_amd64.syso
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

func main() {
	out := flag.String("o", "versioninfo_windows_amd64.syso", "输出的 .syso 文件")
	src := flag.String("src", "brand.go", "产品名/版本号的唯一出处（cmd/app/brand.go）")
	product := flag.String("product", "", "覆盖 ProductName / FileDescription；空则从 -src 读")
	binary := flag.String("binary", "", "覆盖 InternalName / OriginalFilename；空则从 -src 读（含 .exe 的产物名）")
	version := flag.String("version", "", "覆盖版本号；空则从 -src 读")
	company := flag.String("company", "DSH-GO", "CompanyName（项目名，不是两条线各自的名字）")
	flag.Parse()

	if *product == "" {
		*product = decl(*src, `(?m)^const productName\s*=\s*"([^"]+)"`, "productName")
	}
	if *binary == "" {
		*binary = decl(*src, `(?m)^const binaryName\s*=\s*"([^"]+)"`, "binaryName")
	}
	if *version == "" {
		*version = decl(*src, `(?m)^var version\s*=\s*"([^"]+)"`, "version")
	}

	ms, ls, err := fixedVersion(*version)
	if err != nil {
		fail("%v", err)
	}

	blob := versionInfo(*product, *company, *binary, *version, ms, ls)
	section, rvaField := resourceSection(blob)
	obj := coffObject(section, rvaField)
	if err := os.WriteFile(*out, obj, 0o644); err != nil {
		fail("写 %s 失败：%v", *out, err)
	}
	fmt.Printf("VERSIONINFO: %s %s（FileDescription=%s，CompanyName=%s，%s）→ %s（%d 字节）\n",
		*product, *version, *product, *company, *binary, *out, len(obj))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "vsinfo: "+format+"\n", args...)
	os.Exit(1)
}

// decl 从 Go 源码里读一条声明。
//
// 为什么读源码而不是 import：`cmd/app` 是 main 包，导不进来；而把名字抄一份到
// 这个生成器里，就是"两处字面量"——正是 brand.go 要消灭的东西。
func decl(path, pattern, what string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		fail("读 %s 失败：%v", path, err)
	}
	m := regexp.MustCompile(pattern).FindSubmatch(b)
	if m == nil {
		fail("%s 里没有 %s 声明", path, what)
	}
	return string(m[1])
}

// fixedVersion 把版本号拆成 VS_FIXEDFILEINFO 的两个 DWORD。
//
// `0.2.1-go.30` 这种：前三段是语义版本，`-go.N` 的 N 落最低 16 位。
// `dev`（本仓没有 CHANGELOG 时的默认值）：Windows 只认数字，所以填 0.0.0.0——
// **字符串字段仍然是 dev**，任务管理器"描述"读的是 FileDescription，不看这四个数。
func fixedVersion(v string) (ms, ls uint32, err error) {
	if v == "dev" || v == "" {
		return 0, 0, nil
	}
	release, build := v, "0"
	if i := strings.Index(v, "-go."); i >= 0 {
		release, build = v[:i], v[i+len("-go."):]
	}
	parts := strings.Split(release, ".")
	if len(parts) < 3 {
		return 0, 0, fmt.Errorf("版本号 %q 既不是 dev，也不是 x.y.z 形式", v)
	}
	var n [4]uint32
	for i := 0; i < 3; i++ {
		x, e := strconv.ParseUint(parts[i], 10, 16)
		if e != nil {
			return 0, 0, fmt.Errorf("版本号 %q 的第 %d 段不是数字：%v", v, i+1, e)
		}
		n[i] = uint32(x)
	}
	b, e := strconv.ParseUint(build, 10, 16)
	if e != nil {
		return 0, 0, fmt.Errorf("版本号 %q 的构建号不是数字：%v", v, e)
	}
	n[3] = uint32(b)
	return n[0]<<16 | n[1], n[2]<<16 | n[3], nil
}

// --- VERSIONINFO 结构（MS 文档的 BLOCK/StringFileInfo/VarFileInfo） ---

// node 拼一个 VERSIONINFO 块：6 字节头 + UTF-16 键 + 值 + 子块，每段按 4 字节对齐。
//
// wValueLength 的语义按块而变：字符串块是 WCHAR 个数（含结尾 NUL），
// 二进制块（VS_FIXEDFILEINFO、Translation）是字节数，其它为 0。
func node(key string, wType, wValueLength uint16, value []byte, children ...[]byte) []byte {
	var b bytes.Buffer
	putU16(&b, 0) // wLength，最后回填
	putU16(&b, wValueLength)
	putU16(&b, wType)
	b.Write(utf16z(key))
	pad4(&b)
	b.Write(value)
	pad4(&b)
	for _, c := range children {
		b.Write(c)
	}
	pad4(&b)
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out[0:2], uint16(len(out)))
	return out
}

// strNode 是"键 + 字符串值"的块（StringTable 里每一条都是这个形状）。
func strNode(key, val string) []byte {
	v := utf16z(val)
	return node(key, 1, uint16(len(v)/2), v)
}

func versionInfo(product, company, binary, ver string, ms, ls uint32) []byte {
	// StringTable 的键是"语言+代码页"的十六进制：0409 = en-US，04B0 = UTF-16。
	// 值不本地化，用 en-US 是通行做法（任务管理器不看语言，读 FileDescription）。
	table := node("040904B0", 1, 0, nil,
		strNode("CompanyName", company),
		strNode("FileDescription", product),
		strNode("FileVersion", ver),
		// InternalName 按惯例是**不带扩展名**的模块名，OriginalFilename 是
		// **带扩展名**的产物名。两者都从 brand.go 的 binaryName 来（它含 `.exe`），
		// 这里只做一次后缀裁剪——不再各自拼一遍 `.exe`，否则改名成
		// `DSH GUI GO.exe` 之后会生成 `DSH GUI GO.exe.exe`。
		strNode("InternalName", strings.TrimSuffix(binary, ".exe")),
		strNode("OriginalFilename", binary),
		strNode("ProductName", product),
		strNode("ProductVersion", ver),
	)
	stringFileInfo := node("StringFileInfo", 1, 0, nil, table)

	// Translation：LangID(0x0409) + CodePage(0x04B0)，各 2 字节
	var trans bytes.Buffer
	putU16(&trans, 0x0409)
	putU16(&trans, 0x04B0)
	varFileInfo := node("VarFileInfo", 1, 0, nil, node("Translation", 0, 4, trans.Bytes()))

	return node("VS_VERSION_INFO", 0, 52, fixedInfo(ms, ls), stringFileInfo, varFileInfo)
}

// fixedInfo 是 13 个 DWORD 的 VS_FIXEDFILEINFO。
func fixedInfo(ms, ls uint32) []byte {
	fields := []uint32{
		0xFEEF04BD, // dwSignature
		0x00010000, // dwStrucVersion
		ms,         // dwFileVersionMS
		ls,         // dwFileVersionLS
		ms,         // dwProductVersionMS
		ls,         // dwProductVersionLS
		0x3F,       // dwFileFlagsMask
		0,          // dwFileFlags
		0x40004,    // dwFileOS = VOS_NT_WINDOWS32
		1,          // dwFileType = VFT_APP
		0,          // dwFileSubtype
		0,          // dwFileDateMS
		0,          // dwFileDateLS
	}
	b := make([]byte, 0, 52)
	for _, f := range fields {
		b = binary.LittleEndian.AppendUint32(b, f)
	}
	return b
}

// --- 资源段 ---

// resourceSection 排 .rsrc 段。
//
// **目录树必须排在最前**：PE 的 data directory[2]（资源）指向的是段的起点，
// 而链接器只会把那条重定位算成"段首 RVA"。目录树要是排在数据后面，
// 资源管理器读到的就是 VS_VERSIONINFO 的正文（一堆 UTF-16 垃圾）——
// 现象是**二进制里明明有 "DSH GUI GO"，任务管理器的「描述」列却还是空白**。
// 这条不是猜的：第一版就是把数据放在前面，编出来的 exe 逐字搜得到产品名、
// `(Get-Item).VersionInfo` 却全是空，改用目录在前才读得出来。
//
// 目录内所有偏移都相对**段首**；只有数据项里那个 RVA 要留成重定位
// （IMAGE_REL_AMD64_ADDR32NB），由链接器加上段首的运行时地址。
func resourceSection(blob []byte) ([]byte, int) {
	// 三段目录：根（类型）→ 类型 → 名称 → 数据项，每段 16 字节头 + 1 条 8 字节项。
	const (
		rootDir   = 0
		typeDir   = rootDir + dirHeaderSize + dirEntrySize
		nameDir   = typeDir + dirHeaderSize + dirEntrySize
		dataEntry = nameDir + dirHeaderSize + dirEntrySize
		blobAt    = dataEntry + 16 // 数据项本身 16 字节
	)

	var b bytes.Buffer

	// 根目录：0 个命名项，1 个 ID 项；类型 16（RT_VERSION）→ 指向"类型目录"（高位标志=1）
	writeDirHeader(&b, 0, 1)
	writeDirEntry(&b, 16, uint32(typeDir)|0x80000000)

	// 类型目录：ID 项 1 → 指向"名称目录"
	writeDirHeader(&b, 0, 1)
	writeDirEntry(&b, 1, uint32(nameDir)|0x80000000)

	// 名称目录：语言 0x0409 → 指向数据项（**不带**高位标志）
	writeDirHeader(&b, 0, 1)
	writeDirEntry(&b, 0x0409, uint32(dataEntry))

	// 数据项：OffsetToData（段内偏移，运行时由重定位变成 RVA）、大小、代码页、保留
	binary.Write(&b, binary.LittleEndian, uint32(blobAt)) // ← 重定位落点
	binary.Write(&b, binary.LittleEndian, uint32(len(blob)))
	binary.Write(&b, binary.LittleEndian, uint32(0)) // codePage=0 表示不做代码页转换
	binary.Write(&b, binary.LittleEndian, uint32(0))

	// 数据正文：按 4 字节对齐接在目录树后面
	for b.Len() < blobAt {
		b.WriteByte(0)
	}
	b.Write(blob)
	pad4(&b)
	return b.Bytes(), dataEntry
}

const (
	dirHeaderSize = 16
	dirEntrySize  = 8
)

func writeDirHeader(b *bytes.Buffer, named, ids uint16) {
	binary.Write(b, binary.LittleEndian, uint32(0)) // Characteristics
	binary.Write(b, binary.LittleEndian, uint32(0)) // TimeDateStamp
	binary.Write(b, binary.LittleEndian, uint16(0)) // MajorVersion
	binary.Write(b, binary.LittleEndian, uint16(0)) // MinorVersion
	binary.Write(b, binary.LittleEndian, named)
	binary.Write(b, binary.LittleEndian, ids)
}

func writeDirEntry(b *bytes.Buffer, name, target uint32) {
	binary.Write(b, binary.LittleEndian, name)
	binary.Write(b, binary.LittleEndian, target)
}

// --- COFF 目标文件 ---

// coffObject 包一个最小的 COFF 目标文件：一个 `.rsrc` 段、一个段符号、一条重定位。
// 这正是 Go 链接器认的形状。
//
// 那条重定位是**必须**的：资源目录的叶子项里 OffsetToData 存的是运行时 RVA
// （相对映像基址），生成 .syso 的时候根本不知道段会落在哪。没有它，字段就一直是 0，
// 资源管理器读出来就是"没有版本信息"——二进制里能看到 VS_VERSION_INFO 字符串，
// 任务管理器却还是空白。
func coffObject(section []byte, relocOff int) []byte {
	const (
		machineAMD64     = 0x8664
		headerSize       = 20
		sectionHdrSize   = 40
		rawData          = headerSize + sectionHdrSize
		relocSize        = 10
		symSize          = 18
		stringTableLen   = 4          // 只有长度字段，没有长名字
		scnInitData      = 0x40       // IMAGE_SCN_CNT_INITIALIZED_DATA
		scnMemRead       = 0x40000000 // IMAGE_SCN_MEM_READ
		relAMD64Addr32NB = 0x0003     // IMAGE_REL_AMD64_ADDR32NB（RVA）
		symClassStatic   = 3          // IMAGE_SYM_CLASS_STATIC
	)

	data := padTo(section, 4)
	relocAt := rawData + len(data)
	symAt := align4(relocAt + relocSize)
	strAt := symAt + symSize

	var b bytes.Buffer
	// COFF 头
	binary.Write(&b, binary.LittleEndian, uint16(machineAMD64))
	binary.Write(&b, binary.LittleEndian, uint16(1))     // NumberOfSections
	binary.Write(&b, binary.LittleEndian, uint32(0))     // TimeDateStamp
	binary.Write(&b, binary.LittleEndian, uint32(symAt)) // PointerToSymbolTable
	binary.Write(&b, binary.LittleEndian, uint32(1))     // NumberOfSymbols
	binary.Write(&b, binary.LittleEndian, uint16(0))     // SizeOfOptionalHeader
	binary.Write(&b, binary.LittleEndian, uint16(0))     // Characteristics

	// 段头
	name := [8]byte{'.', 'r', 's', 'r', 'c'}
	b.Write(name[:])
	binary.Write(&b, binary.LittleEndian, uint32(len(data))) // VirtualSize
	binary.Write(&b, binary.LittleEndian, uint32(0))         // VirtualAddress
	binary.Write(&b, binary.LittleEndian, uint32(len(data))) // SizeOfRawData
	binary.Write(&b, binary.LittleEndian, uint32(rawData))   // PointerToRawData
	binary.Write(&b, binary.LittleEndian, uint32(relocAt))   // PointerToRelocations
	binary.Write(&b, binary.LittleEndian, uint32(0))         // PointerToLinenumbers
	binary.Write(&b, binary.LittleEndian, uint16(1))         // NumberOfRelocations
	binary.Write(&b, binary.LittleEndian, uint16(0))         // NumberOfLinenumbers
	binary.Write(&b, binary.LittleEndian, uint32(scnInitData|scnMemRead))

	// 段数据
	b.Write(data)

	// 重定位：数据项里的 RVA 字段 ← 本段段首符号（值为 0）
	binary.Write(&b, binary.LittleEndian, uint32(relocOff)) // VirtualAddress
	binary.Write(&b, binary.LittleEndian, uint32(0))        // SymbolTableIndex
	binary.Write(&b, binary.LittleEndian, uint16(relAMD64Addr32NB))

	// 符号表：一个段符号（STORAGE CLASS = STATIC、段号 1、值 0 ⇒ 链接器认成"段符号"）
	for b.Len() < symAt {
		b.WriteByte(0)
	}
	b.Write([]byte(".rsrc\x00\x00\x00"))
	binary.Write(&b, binary.LittleEndian, uint32(0)) // Value
	binary.Write(&b, binary.LittleEndian, int16(1))  // SectionNumber
	binary.Write(&b, binary.LittleEndian, uint16(0)) // Type
	b.WriteByte(symClassStatic)                      // StorageClass
	b.WriteByte(0)                                   // NumberOfAuxSymbols

	// 字符串表：只有长度字段（长度含自身）
	for b.Len() < strAt {
		b.WriteByte(0)
	}
	binary.Write(&b, binary.LittleEndian, uint32(stringTableLen))

	return b.Bytes()
}

// --- 小工具 ---

func utf16z(s string) []byte {
	u := utf16.Encode([]rune(s))
	b := make([]byte, (len(u)+1)*2) // 末尾两个 0 字节 = NUL 终结
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	return b
}

func putU16(b *bytes.Buffer, v uint16) { binary.Write(b, binary.LittleEndian, v) }

func pad4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func padTo(b []byte, n int) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	for len(out)%n != 0 {
		out = append(out, 0)
	}
	return out
}

func align4(n int) int { return (n + 3) &^ 3 }
