# Run and debug

**Use the Dev Container** so the IDE uses the container's Go and Delve. After opening in the container, **reload the window** once (Command Palette → "Developer: Reload Window") so the Go extension picks up the environment.

## How to run tests

1. **Run and Debug view**  
   - Click the **Run and Debug** icon in the left sidebar (play icon with bug), or press `Ctrl+Shift+D` / `Cmd+Shift+D`.  
   - Select **"Run Tests"** and press **F5** to run all tests with the debugger.

2. **Task (run in terminal)**  
   `Ctrl+Shift+P` / `Cmd+Shift+P` → **Tasks: Run Task** → **Run Tests**.  
   This runs `go test ./...` in the integrated terminal.

3. **Debug a single test**  
   Use **"Debug Current Test"** or **"Debug Test File"** from the Run and Debug dropdown, or use the **Run** | **Debug** codelens above a test function.

## Keyboard shortcuts

- **F5** – Start with debugger (breakpoints work).
- **Ctrl+F5** / **Cmd+F5** – Run without debugger.

## If run/debug doesn't work

- Confirm you're **inside the Dev Container** (e.g. "Dev Containers: Reopen in Container"), then **reload the window** (Command Palette → "Developer: Reload Window").
- Install/update Go tools in the container: Command Palette → **Go: Install/Update Tools** → select all → OK.
- Check the **Debug Console** and **Go Output** (View → Output → "Go") for errors.
