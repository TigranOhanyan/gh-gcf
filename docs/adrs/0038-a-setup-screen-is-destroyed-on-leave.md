# A setup screen is destroyed on leave

The TUI shows one screen. That screen is its own model. Its update reads its own fields and builds the next screen. The screen that was left is dropped, and a return builds a new one, reading again any files that screen needs and closing them. A form is one screen. The screen draws its breadcrumb and its key line. A save builds the next list with the saved row selected. Any other visit starts at the first row. A failed edit stays on the same screen.

This supersedes `0031-setup-screens-are-stable-widgets.md`.

## Considered Options

- One model and a screen enum: rejected, because the fields of every screen stayed on the model while another screen was showing.
- Stable widgets that keep parent and child pointers: rejected, because a screen that is off should not exist.
- A frame that holds the current screen and draws the breadcrumb and the key line: rejected, because the screen that handled the key builds the next screen, and the frame would have to move the text cursor.
- A stack of screens: rejected, because back would show the old screen instead of building it again.
