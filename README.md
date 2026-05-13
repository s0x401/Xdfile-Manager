# Xdfile Manager

Xdfile Manager 是一个双面板终端文件管理器，面向键盘高效操作，同时补齐常用鼠标交互。它支持本地文件管理、内置命令行、预览、用户菜单、主题和 SSH / NetBox 远程目录。

推荐在 Windows Terminal 中使用：

https://github.com/microsoft/terminal

## 启动

Windows：

```powershell
dist\windows-amd64\xdfile.exe
```

Linux：

```bash
chmod +x dist/linux-amd64/xdfile
dist/linux-amd64/xdfile
```

指定左右面板起始目录：

```powershell
xdfile.exe C:\work D:\data
```

```bash
./xdfile ~/work ~/data
```

查看配置路径：

```powershell
xdfile.exe path-list
```

## 参数

| 参数 | 功能 |
|---|---|
| `-c, --config-file` | 指定配置文件 |
| `-hf, --hotkey-file` | 指定快捷键文件 |
| `-cf, --chooser-file` | 输出打开路径 |
| `-pld, --print-last-dir` | 输出最后目录 |
| `-fh, --fix-hotkeys` | 补齐快捷键 |
| `-fch, --fix-config-file` | 补齐主配置 |

## 界面

| 区域 | 功能 |
|---|---|
| 顶部 | 主菜单、路径、状态 |
| 左右面板 | 文件列表 |
| 底部终端 | 命令输入和输出 |
| 底栏 | 当前选择、面板、排序和状态信息 |

状态符号：

| 符号 | 状态 |
|---|---|
| `✓` | 完成 |
| `!` | 失败 |
| `?` | 等待 |
| `×` | 取消 |
| `◇` | 普通状态 |
| `□□□□□□□□□□` | 执行中 |

## 基础操作

| 快捷键 / 操作 | 功能 |
|---|---|
| `Tab / Shift+Tab` | 切换左右面板焦点 |
| `Enter` | 进入目录或打开文件 |
| `Up / Down` | 移动文件光标 |
| `Left / Right` | 按页移动文件光标 |
| `PgUp / PgDn` | 翻页 |
| `Home / End` | 跳到首项 / 末项 |
| `Esc` | 清除选择或关闭当前弹窗 |
| `R` | 刷新 |
| `F1` | 帮助 |
| `F9` | 显示 / 隐藏隐藏文件 |
| `F10` | 退出 |
| `Ctrl+Left / Ctrl+Right` | 调整左右面板宽度 |
| `Ctrl+Up / Ctrl+Down` | 调整终端高度 |
| `Ctrl+3` | 按名称排序 |
| `Ctrl+4 / Ctrl+\` | 按扩展名排序 |

## 鼠标

| 操作 | 功能 |
|---|---|
| 左键单击 | 聚焦面板并选择文件 |
| 双击 | 进入目录或打开文件 |
| 右键 | 打开与面板样式一致的 TUI 右键菜单 |
| `Ctrl + 左键` | 切换单项多选 |
| `Shift + 左键` | 范围选择，取决于终端是否传递 Shift 鼠标事件 |
| `Alt + 左键` | 范围选择，作为 Windows Terminal 中 Shift 冲突的替代方式 |
| 拖动 | 范围拖选 |
| 滚轮 | 滚动鼠标所在面板，不移动文件光标 |

Windows 本地文件的 TUI 右键菜单中有 `Windows menu` 项。它会调用 Windows Shell 原生右键菜单，支持 7-Zip、Bandizip、杀毒扫描、发送到等系统扩展。原生菜单使用 Windows 系统样式，不能套用 TUI 面板主题；默认右键先打开 TUI 菜单，是为了保持界面风格统一。

Windows 原生菜单限制：

- 只作用于本地文件或目录。
- 多选项需要位于同一目录。
- 远程路径、`..` 和空白区域继续使用 TUI 菜单。

## 文件操作

| 快捷键 / 操作 | 功能 |
|---|---|
| `Ctrl+Shift+C` | 复制选中文件到系统剪贴板 |
| `Ctrl+X` | 剪切选中文件到系统剪贴板 |
| `Ctrl+Shift+V` | 粘贴 |
| `F4` | 重命名 |
| `F5` | 复制到另一面板 |
| `F6` | 移动到另一面板 |
| `F7` | 新建目录 |
| `F8` | 删除 |
| `Ctrl+Z` | 撤回最近一次删除或面板内剪切移动 |
| `Ctrl+C` | 取消正在执行的文件操作 |

复制、移动、删除、重命名和新建目录会通过后台文件操作队列执行，避免长时间阻塞面板。批量操作会汇总失败项，并在错误弹窗关闭后继续队列。

复制、剪切、删除等操作会优先作用于多选标记项；没有多选时作用于当前光标项。

## 粘贴冲突

| 选项 | 功能 |
|---|---|
| `Replace` | 覆盖 |
| `Skip` | 跳过 |
| `Keep both` | 保留两份 |
| `Apply all` | 后续冲突使用同一策略 |

Linux 文件剪贴板需要 `wl-clipboard`、`xclip` 或 `xsel`。

## 多选

| 快捷键 / 操作 | 功能 |
|---|---|
| `Shift+Up / Shift+Down` | 扩展当前范围选择 |
| `Shift+Left` | 扩展选择到第一项 |
| `Shift+Right` | 扩展选择到最后一项 |
| `Ctrl + 左键` | 切换单项选择 |
| `Shift / Alt + 左键` | 范围选择 |
| 鼠标拖动 | 范围拖选 |
| `Esc` | 清除选择 |

键盘和鼠标共用同一套选择数据结构，面板操作会一致地读取当前多选状态。

## 快速搜索

| 快捷键 | 功能 |
|---|---|
| `Alt+字符` | 开始搜索 |
| 继续输入 | 继续匹配 |
| `Backspace` | 删除搜索字符 |
| `Enter` | 打开匹配项 |
| `Ctrl+N / Ctrl+P` | 下一个 / 上一个匹配 |
| `Esc / F10` | 关闭搜索 |

支持 `*` 和 `?` 通配符。

## 预览

| 快捷键 / 操作 | 功能 |
|---|---|
| `F3` | 预览当前文件 |
| `Ctrl+Q` | 切换 Quick View |
| `Ctrl+B` | 在预览中切换二进制视图 |
| 滚轮 / `PgUp / PgDn` | 滚动预览内容 |

支持文本、代码、目录、归档、图片、PDF 和二进制内容预览。

## 命令行

| 快捷键 / 操作 | 功能 |
|---|---|
| `Enter` | 执行命令 |
| `PgUp / PgDn` | 滚动输出 |
| `Ctrl+O` | 展开终端 |
| 鼠标点击输入行 | 移动输入光标 |
| `Up / Down` | 选择预测 |
| `Right / Tab` | 接受预测 |
| `Esc` | 关闭预测 |

提示符：

| 提示符 | 功能 |
|---|---|
| `XD>` | 本地命令 |
| `user@连接名>` | 远程命令 |

内置命令包括 `ls`、`ll`、`la`、`cat`、`clear`、`cls`。交互式程序会进入独占终端模式。

## 菜单

| 菜单 | 功能 |
|---|---|
| `Panels` | 面板操作 |
| `View` | 显示和排序 |
| `Terminal` | 终端 |
| `NetBox` | SSH |
| `Theme` | 主题 |
| `Options` | 设置 |

## F2 用户菜单

| 快捷键 | 功能 |
|---|---|
| `F2` | 打开 User Menu |
| `Enter` | 执行或打开子菜单 |
| `Ins` | 新增命令或子菜单 |
| `F4` | 编辑当前项 |
| `Del` | 删除当前项 |
| `Esc` | 返回 |
| `Left / Right` | 切换层级 |

常用 metasymbol：

| 符号 | 功能 |
|---|---|
| `!.!` | 当前文件 |
| `!&` | 选中文件 |
| `!@!` | 选中文件列表 |
| `!?提示?默认值?` | 输入参数 |
| `!# / !^` | 另一面板 |

## SSH / NetBox

新建连接：

```text
NetBox -> New SSH connection
```

| 字段 | 功能 |
|---|---|
| `Name` | 连接名 |
| `Host` | 主机地址 |
| `Port` | 端口 |
| `User` | 用户 |
| `Password` | 密码 |
| `Save password` | 保存密码 |

远程操作：

| 操作 | 功能 |
|---|---|
| 选择连接 | 进入远程目录 |
| `exit / logout` | 断开 |
| `cd` | 切换目录 |
| 其他命令 | 远程执行 |
| `Ctrl+Shift+C` | 复制远程文件 |
| `Ctrl+Shift+V` | 粘贴到远程目录 |
| `F7` | 新建远程目录 |
| `F4` | 远程重命名 |
| `R` | 刷新远程目录 |

远程限制：

- 远程复制 / 粘贴需要远端有 `tar`。
- 远程粘贴只支持复制，不支持剪切。
- 暂不支持远程撤回、剪切、预览和属性。
- 暂不支持本地与远程之间的 `F5 / F6` 面板对面板复制移动。

## 主题

主题菜单：`Theme`

内置主题：

- Persona 3
- Persona 3 Reload
- Persona 3 Kotone
- Persona 4
- Persona 5

## 保存设置

| 操作 | 功能 |
|---|---|
| `Shift+F9` | 保存设置 |
| `Options -> Save setup` | 保存设置 |
| `Options -> Reset setup` | 重置布局、主题、视图选项和用户菜单 |

## 数据目录

程序数据位于可执行文件同目录的 `xdfile-data`。

| 文件 / 目录 | 功能 |
|---|---|
| `xdfile-config.toml` | 主配置 |
| `xdfile-hotkeys.toml` | 快捷键 |
| `xdfile-layout.json` | 布局 |
| `xdfile-commands.json` | User Menu |
| `xdfile-netbox.json` | SSH |
| `xdfile.log` | 日志 |
| `xdfile-lastdir` | 最后目录 |
| `xdfile-theme/` | 主题 |
| `cache/` | 缓存 |

## 构建

Windows：

```powershell
go test ./...
go vet ./...
go build -o dist\windows-amd64\xdfile.exe .
Copy-Item README.md dist\windows-amd64\README.md -Force
```

Linux：

```bash
go test ./...
go vet ./...
go build -o dist/linux-amd64/xdfile .
cp README.md dist/linux-amd64/README.md
chmod +x dist/linux-amd64/xdfile
```

Windows 交叉构建 Linux：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -o dist\linux-amd64\xdfile .
Copy-Item README.md dist\linux-amd64\README.md -Force
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
```

## 许可

AGPL-3.0 license

Copyright (c) 2026 s0x401
