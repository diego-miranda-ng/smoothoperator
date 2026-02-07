# Run and debug the application

**Use the Dev Container** so the IDE uses the container’s Go and Delve (run/debug will not work correctly from the host when the app is meant to run in Docker). After opening in the container, **reload the window** once (Command Palette → “Developer: Reload Window”) so the Go extension picks up the environment.

## How to run and debug

1. **Run and Debug view**  
   - Click the **Run and Debug** icon in the left sidebar (play icon with bug), or press `Ctrl+Shift+D` / `Cmd+Shift+D`.  
   - In the dropdown at the top, select **“Launch Application”** (or **“Launch Application (auto)”** if the first fails).  
   - Press the **green play** button, or **F5** to debug / **Ctrl+F5** (Windows/Linux) or **Cmd+F5** (macOS) to run without debugging.

2. **Status bar**  
   At the bottom, you may see the current launch configuration (e.g. “Launch Application”). Use the **play icon** next to it to run.

3. **Task (run in terminal, no debugger)**  
   `Ctrl+Shift+P` / `Cmd+Shift+P` → **Tasks: Run Task** → **Run Application**.  
   This runs `go run ./cmd/example` in the integrated terminal.

4. **From the main file**  
   Open `cmd/example/main.go`. If the Go extension shows **Run** | **Debug** above `func main()`, click **Debug** to start with breakpoints.

## Keyboard shortcuts

- **F5** – Start with debugger (breakpoints work).
- **Ctrl+F5** / **Cmd+F5** – Run without debugger.

## If run/debug still doesn’t work

- Confirm you’re **inside the Dev Container** (e.g. “Dev Containers: Reopen in Container”), then **reload the window** (Command Palette → “Developer: Reload Window”).
- In the Run and Debug view, try **“Launch Application (auto)”** instead of “Launch Application”.
- Install/update Go tools in the container: Command Palette → **Go: Install/Update Tools** → select all → OK.
- Check the **Debug Console** and **Go Output** (View → Output → “Go”) for errors.
