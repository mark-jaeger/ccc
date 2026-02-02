# Notification Chain: How a Bell Reaches macOS

When Claude Code finishes a task inside a CCC-managed tmux session, the user
needs to know -- even if they're in another window or looking away. This
document explains how a BEL character emitted by the program travels through
tmux (and optionally SSH) to produce a macOS notification.

## Design Principle

CCC does **not** try to detect when a program finishes. Instead, it configures
tmux to faithfully forward bell signals that the program itself emits. Claude
Code already emits a BEL (`\a`) when it finishes processing and shows its input
prompt. CCC's job is to make sure that byte reaches the terminal.

An earlier design used tmux's `monitor-silence` to detect 5 seconds of no
output and generate a bell. This was removed because Claude Code regularly goes
silent for 10-30+ seconds while thinking, producing false notifications before
the task is actually done.

## The Setup

When CCC creates or attaches to a tmux session, it configures these options
(`tmux/sessions.go`):

| Option | Value | Purpose |
|---|---|---|
| `bell-action` | `any` | Forward bell signals from any window to the client |
| `visual-bell` | `off` | Send a real BEL byte, not a status-bar flash |
| `allow-passthrough` | `on` | Let escape sequences through tmux (tmux >= 3.3) |

That's it. Three options.

## The Signal Chain

```
YOUR MAC                           REMOTE SERVER
┌──────────────┐                   ┌──────────────────────────────────┐
│              │                   │  ┌────────────────────────────┐  │
│              │                   │  │  tmux session              │  │
│              │                   │  │  ┌──────────────────────┐  │  │
│              │      SSH          │  │  │                      │  │  │
│  Terminal ◄──┼──── tunnel ◄──────┼──┼──┤  Claude Code         │  │  │
│  (iTerm2)    │                   │  │  │                      │  │  │
│              │                   │  │  │  ... working ...      │  │  │
│              │                   │  │  │  Task complete.       │  │  │
│              │                   │  │  │  > _  (emits \a)     │  │  │
│              │                   │  │  │       |               │  │  │
└──────┬───────┘                   │  │  └───────┼──────────────┘  │  │
       │                           │  │          |                 │  │
       v                           │  │          v                 │  │
  macOS sees it                    │  │  tmux forwards BEL         │  │
                                   │  └──────────┼────────────────┘  │
                                   │             │                   │
                                   └─────────────┼───────────────────┘
                                                 │
                                        (see steps below)
```

## Step by Step

### Step 1 -- Claude Code finishes and emits BEL

Claude Code completes its response and shows the input prompt. As part of this,
it writes a BEL character (`\a`, ASCII 0x07) to stdout. This is the same
mechanism that makes terminals bounce their dock icon or play a sound.

### Step 2 -- tmux forwards the bell

`bell-action any` tells tmux to forward bells from any window in the session to
the attached client. Without this, tmux would swallow the bell if the user were
looking at a different window.

`visual-bell off` ensures tmux sends the actual BEL byte to the terminal's PTY
rather than just showing a message in the status bar.

### Step 3 -- BEL travels through SSH

*(This step only applies in remote mode.)*

CCC attached to the remote tmux session using an interactive SSH channel with
PTY allocation (`ssh -t`). The BEL byte is just data in the encrypted SSH
stream -- SSH delivers `0x07` to the local terminal without interpreting it.

```
Remote PTY ──> SSH encrypted stream ──> Local PTY
    \a                                      \a
```

### Step 4 -- Terminal emulator receives BEL

The terminal emulator (iTerm2, Terminal.app, etc.) receives `0x07` and reacts
according to its own settings -- typically one or more of:

- Bounce the dock icon
- Post to macOS Notification Center
- Play a sound

The terminal calls macOS notification APIs to surface the alert.

### Step 5 -- macOS shows the notification

A banner appears in Notification Center, the dock icon bounces, and/or a sound
plays. The user switches back to their terminal and sees Claude Code's output.

## Escape Sequence Passthrough

Beyond the BEL mechanism, CCC enables `allow-passthrough on`. This lets OSC
escape sequences pass through tmux to the terminal. Modern CLI tools can emit
these sequences to trigger rich notifications with custom text -- normally tmux
strips them out.

This option requires tmux >= 3.3. CCC sets it in a separate command with errors
ignored so that session creation still works on older tmux versions. The
degradation is graceful: older tmux still gets BEL notifications, just not rich
OSC notifications.

## Design Decisions

**Why `visual-bell off`?** The name is misleading. In tmux, `visual-bell on`
means "show a message in the status bar *instead of* sending BEL." That would
swallow the signal before it reaches the terminal. `off` means "send the real
byte."

**Why not `monitor-silence`?** An earlier version used tmux's `monitor-silence`
to detect 5 seconds of no output and generate a bell. This was removed because
Claude Code regularly pauses for 10-30+ seconds while thinking, causing false
notifications. The correct approach is to let the program signal when it's
actually done, rather than guessing based on silence.

**Why is passthrough set separately?** `allow-passthrough` was added in tmux
3.3. Including it in the main create command would break session creation on
older versions. A separate command with ignored errors provides graceful
degradation.

**Why apply options on every attach?** `BuildEnsureNotifyOptionsCommand` re-
applies all settings idempotently on every `tmux attach`. This fixes sessions
created by older CCC versions that may be missing some options.

## Terminal Configuration

For the notification to reach macOS, your terminal must be configured to act on
BEL characters:

**iTerm2:** Profiles -> Terminal -> Notifications -> check "Show bell icon in
tabs" and/or enable "Notification center alerts."

**Terminal.app:** Settings -> Profiles -> Advanced -> Bell -> select "Bounce
dock icon" or "Visual bell."
