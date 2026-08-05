# Regenerates the README screenshots with https://github.com/charmbracelet/vhs.
#
#   go build -o /tmp/irori-shot ./cmd/irori
#   cd <a server directory>            # the shots below use a Paper 1.21.4 dir
#   PATH=/tmp:$PATH vhs path/to/preview.tape
#
# Then copy shot-configs.png over .github/branding/preview.png, and the plugins
# and settings shots over their own files.

Output "preview.gif"

Set Shell "bash"
Set FontSize 15
Set Width 1600
Set Height 900
Set Padding 30
Set Theme "Catppuccin Mocha"

Hide
Type "clear"
Enter
Sleep 1s
Show

Type "irori-shot"
Sleep 400ms
Enter
Sleep 4s

# The console input has focus on start, so digits would be typed into it.
Tab
Sleep 1s
Screenshot "shot-console.png"

Type "2"
Sleep 2500ms
Screenshot "shot-files.png"

Type "3"
Sleep 2500ms
Screenshot "shot-plugins.png"

Type "4"
Sleep 3s
Screenshot "shot-configs.png"

Type "5"
Sleep 2500ms
Screenshot "shot-settings.png"
Sleep 500ms
