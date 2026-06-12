//go:build e2e_mock_llm

package providers

func testOnlyConfigForProvider(id string) Config {
	if id == "e2e-mock-llm" {
		return Config{"enabled": "true"}
	}
	return nil
}
