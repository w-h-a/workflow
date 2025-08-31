package containerlogs

import (
	"bufio"
	"fmt"
	"net/http"
)

func RunStreamerClient(client *http.Client, id string) {
	url := fmt.Sprintf("http://localhost:4002/logs/%s", id)

	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("Error making request:", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Server returned non-200 status: %s\n", resp.Status)
		return
	}

	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading from stream:", err)
	}

	fmt.Println("Connection closed.")
}
