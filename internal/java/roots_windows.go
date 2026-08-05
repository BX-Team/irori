//go:build windows

package java

const javaBin = "java.exe"

var systemGlobs = []string{
	`C:\Program Files\Java\*\bin\java.exe`,
	`C:\Program Files\Eclipse Adoptium\*\bin\java.exe`,
	`C:\Program Files\Microsoft\*\bin\java.exe`,
	`C:\Program Files\Zulu\*\bin\java.exe`,
	`C:\Program Files\Amazon Corretto\*\bin\java.exe`,
	`C:\Program Files\BellSoft\*\bin\java.exe`,
	`C:\Program Files\Semeru\*\bin\java.exe`,
}
