# Xdfile Manager

双栏终端文件管理器，支持本地文件、命令行、预览、F2 宏命令、主题和 SSH / NetBox。

### 推荐Windows Terminal + Xdfile Manager组合使用

https://github.com/microsoft/terminal

## 启动

Windows：

```bash
dist\windows-amd64\xdfile.exe
```

Linux：

```bash
chmod +x dist/linux-amd64/xdfile
dist/linux-amd64/xdfile
```

指定左右目录：

```bash
xdfile.exe C:\work D:\data
./xdfile ~/work ~/data
```

查看配置路径：

```bash
xdfile.exe path-list
```

## 参数

| 参数 | 功能 |
|---|---|
| `-c, --config-file` | 指定配置 |
| `-hf, --hotkey-file` | 指定快捷键 |
| `-cf, --chooser-file` | 输出打开路径 |
| `-pld, --print-last-dir` | 输出最后目录 |
| `-fh, --fix-hotkeys` | 补齐快捷键 |
| `-fch, --fix-config-file` | 补齐主配置 |

## 界面

| 区域 | 功能 |
|---|---|
| 顶部 | 菜单、路径、状态 |
| 左右面板 | 文件列表 |
| 底部终端 | 命令输出和输入 |
| 底栏 | 选择信息和功能键 |

状态符号：

| 符号 | 状态 |
|---|---|
| `✓` | 完成 |
| `!` | 失败 |
| `?` | 等待 |
| `×` | 取消 |
| `●` | 普通 |
| `⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏` | 执行中 |

## 基础操作

| 快捷键 / 操作 | 功能 |
|---|---|
| `Tab / Shift+Tab` | 切换焦点 |
| `Enter` | 打开 |
| `Up / Down` | 移动光标 |
| `Left / Right` | 按页移动 |
| `PgUp / PgDn` | 翻页 |
| `Home / End` | 首项 / 末项 |
| `Esc` | 取消 |
| `R` | 刷新 |
| `F1` | 帮助 |
| `F9` | 显示隐藏文件 |
| `F10` | 退出 |
| `Ctrl+Left / Ctrl+Right` | 调整面板宽度 |
| `Ctrl+Up / Ctrl+Down` | 调整终端高度 |
| `Ctrl+3` | 按名称排序 |
| `Ctrl+4 / Ctrl+\` | 按扩展名排序 |

## 鼠标

| 操作 | 功能 |
|---|---|
| 左键 | 选择 |
| 双击 | 打开 |
| 右键 | 菜单 |
| 滚轮 | 滚动 |

## 弹窗

| 快捷键 | 功能 |
|---|---|
| `Left / Right / Tab` | 切换选项 |
| `Enter` | 确认 |
| `Esc` | 取消 |

## 文件

| 快捷键 | 功能 |
|---|---|
| `Ctrl+Shift+C` | 复制 |
| `Ctrl+X` | 剪切 |
| `Ctrl+Shift+V` | 粘贴 |
| `F4` | 重命名 |
| `F5` | 复制到另一面板 |
| `F6` | 移动到另一面板 |
| `F7` | 新建目录 |
| `F8` | 删除 |
| `Ctrl+Z` | 撤回 |

## 粘贴冲突

| 选项 | 功能 |
|---|---|
| `Replace` | 覆盖 |
| `Skip` | 跳过 |
| `Keep both` | 保留两份 |
| `Apply all` | 应用全部 |

Linux 剪贴板需要 `wl-clipboard`、`xclip` 或 `xsel`。

## 多选

| 快捷键 | 功能 |
|---|---|
| `Shift+Up / Shift+Down` | 标记并移动 |
| `Shift+Left` | 选择到上方 |
| `Shift+Right` | 选择到下方 |
| `Esc` | 清除选择 |

复制、剪切、删除会优先作用于标记项。

## 快速搜索

| 快捷键 | 功能 |
|---|---|
| `Alt+字符` | 搜索 |
| 继续输入 | 继续匹配 |
| `Backspace` | 删除字符 |
| `Enter` | 打开匹配项 |
| `Ctrl+N / Ctrl+P` | 下一个 / 上一个 |
| `Esc / F10` | 关闭 |

支持 `*` 和 `?`。

## 预览

| 快捷键 | 功能 |
|---|---|
| `F3` | 预览 |
| `Ctrl+Q` | Quick View |
| `Ctrl+B` | 二进制视图 |
| `滚轮 / PgUp / PgDn` | 滚动 |

支持文本、代码、目录、归档、图片、PDF、二进制。

## 命令行

| 快捷键 / 操作 | 功能 |
|---|---|
| `Enter` | 执行 |
| `PgUp / PgDn` | 滚动输出 |
| `Ctrl+O` | 放大终端 |
| 鼠标点击输入行 | 移动光标 |
| `Up / Down` | 选择预测 |
| `Right / Tab` | 接受预测 |
| `Esc` | 关闭预测 |

提示符：

| 提示符 | 功能 |
|---|---|
| `XD>` | 本地命令 |
| `user@连接名>` | 远程命令 |

内置命令：`ls`、`ll`、`la`、`cat`、`clear`、`cls`。

交互程序会进入独占终端。

## 菜单

| 菜单 | 功能 |
|---|---|
| `Panels` | 面板操作 |
| `View` | 显示和排序 |
| `Terminal` | 终端 |
| `NetBox` | SSH |
| `Theme` | 主题 |
| `Options` | 设置 |

## F2 宏命令

| 快捷键 | 功能 |
|---|---|
| `F2` | 打开 User Menu |
| `Enter` | 执行 |
| `Ins` | 新增 |
| `F4` | 编辑 |
| `Del` | 删除 |
| `Esc` | 返回 |
| `Left / Right` | 切换层级 |

常用 metasymbol：

| 符号 | 功能 |
|---|---|
| `!.!` | 当前文件 |
| `!&` | 选中文件 |
| `!@!` | 选中文件列表 |
| `!?提示?默认值!` | 输入参数 |
| `!# / !^` | 另一面板 |

## SSH / NetBox

新建连接：

```text
NetBox -> New SSH connection
```

| 字段 | 功能 |
|---|---|
| `Name` | 名称 |
| `Host` | 地址 |
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
| `Ctrl+Shift+V` | 粘贴到远程 |
| `F7` | 新建远程目录 |
| `F4` | 远程重命名 |
| `R` | 刷新远程目录 |

远程限制：

- 远程复制 / 粘贴需要 `tar`
- 远程粘贴只支持复制
- 暂不支持远程撤回、剪切、预览、属性
- 暂不支持本地和远程之间的 `F5 / F6`

## 主题

主题菜单：`Theme`

内置主题：

- Persona 3
- Persona 3 Reload
- Persona 3 Kotone
- Persona 4
- Persona 5

## 保存

| 操作 | 功能 |
|---|---|
| `Shift+F9` | 保存设置 |
| `Options -> Save setup` | 保存设置 |

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
go build -o dist\windows-amd64\xdfile.exe .
Copy-Item README.md dist\windows-amd64\README.md -Force
```

Linux：

```bash
go test ./...
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
```

## 许可证

AGPL-3.0 license

Copyright (c) 2026 s0x401
