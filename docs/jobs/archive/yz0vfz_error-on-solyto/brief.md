# Brief: error on solyto

status: done
type: feature
id: yz0vfz
branch: feature/yz0vfz_error-on-solyto
date: 2026-08-12
author: Leander Muskalla

## What

I tried using mg in one of my projects.
I ran mg init, all gone through.
Then wanted to run mg tui and got this error:

Caught panic:

runtime error: slice bounds out of range [:1] with capacity 0

Restoring terminal...

goroutine 1 [running]:
runtime/debug.Stack()
	runtime/debug/stack.go:26 +0x5e
runtime/debug.PrintStack()
	runtime/debug/stack.go:18 +0x13
github.com/charmbracelet/bubbletea.(*Program).recoverFromPanic(0xc000252780)
	github.com/charmbracelet/bubbletea@v1.2.4/tea.go:705 +0x8b
panic({0x8e3fa0?, 0xc0002fe120?})
	runtime/panic.go:783 +0x132
github.com/lmuskalla/manigot/internal/ui.(*App).renderRecentActivity(0xeda580?, 0x40?)
	github.com/lmuskalla/manigot/internal/ui/app.go:1236 +0x78d
github.com/lmuskalla/manigot/internal/ui.(*App).renderList(0xc000252280)
	github.com/lmuskalla/manigot/internal/ui/app.go:1204 +0xf78
github.com/lmuskalla/manigot/internal/ui.(*App).View(0xc00047c000?)
	github.com/lmuskalla/manigot/internal/ui/app.go:398 +0x6b
github.com/charmbracelet/bubbletea.(*Program).Run(0xc000252780)
	github.com/charmbracelet/bubbletea@v1.2.4/tea.go:579 +0x8a2
main.runTUI({0xc000014070, 0x0, 0x0}, {0xeddd60?, 0xc00002a070?}, {0xbb2748, 0xc000072030})
	github.com/lmuskalla/manigot/cmd/mg/tui.go:74 +0x2de
main.main()
	github.com/lmuskalla/manigot/cmd/mg/main.go:44 +0x1dc


## Why

<!-- Why does this need to exist? What problem does it solve for the user? -->

## Out of scope

<!-- What are we explicitly NOT doing in this job? -->

## Notes

<!-- Anything the analyst or developer should know before starting. -->

