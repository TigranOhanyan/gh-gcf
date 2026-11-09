# Setup screens are stable widgets

Superseded by `0038-a-setup-screen-is-destroyed-on-leave.md`. A screen is dropped on leave and built again on return.

Setup's program model holds pointers to widgets. A widget is allocated once and is not copied. `Update` copies the program model and replaces the page value inside the current widget. Widgets point at their parent and their children. A `noCopy` sentinel makes `go vet` reject a widget copied by value. Copying the pointer stays legal. The screen enum and the switches that restate its parent links go away.

## Considered Options

- A stack of page values: rejected, because a new screen still has no stable parent to hang from.
- Parent and child pointers stored on the page value: rejected, because `Update` returns a new value and those pointers would aim at the previous copy.
