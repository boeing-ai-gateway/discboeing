//go:build !e2e_mock_llm

package providers

func testOnlyConfigForProvider(string) Config {
	return nil
}
