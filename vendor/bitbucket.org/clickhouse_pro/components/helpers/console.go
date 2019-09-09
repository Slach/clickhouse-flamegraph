package helpers

import (
	"bufio"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"strings"
)

//ReadInputConsoleString helper show prompt and return keyboard input
func ReadInputConsoleString(msg string, params ...interface{}) string {
	consolereader := bufio.NewReader(os.Stdin)

	fmt.Printf(msg, params...)
	result, err := consolereader.ReadString('\n')
	if err != nil {
		log.Fatal().Err(err)
	}
	return strings.Replace(result, string('\n'), "", -1)
}
