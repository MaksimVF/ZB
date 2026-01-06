


import unittest
import os
import json
from unittest.mock import patch, MagicMock
from server import call_litellm, ModelServicer, get_provider_keys_from_secrets

class TestModelProxy(unittest.TestCase):

    def setUp(self):
        # Set up test environment
        os.environ["PROVIDER_KEYS"] = json.dumps({
            "openai": "test-openai-key",
            "anthropic": "test-anthropic-key",
            "mistral": "test-mistral-key",
            "groq": "test-groq-key"
        })

    def test_get_provider_keys_validation(self):
        """Test that provider keys are properly validated"""
        with patch('server.logger') as mock_logger:
            provider_keys = get_provider_keys_from_secrets()

            # Check that all required providers are present
            self.assertIn("openai", provider_keys)
            self.assertIn("anthropic", provider_keys)
            self.assertIn("mistral", provider_keys)
            self.assertIn("groq", provider_keys)

            # Check that keys are not empty
            self.assertTrue(provider_keys["openai"])
            self.assertTrue(provider_keys["anthropic"])
            self.assertTrue(provider_keys["mistral"])
            self.assertTrue(provider_keys["groq"])

            # Check that masking was logged
            mock_logger.info.assert_called_with("Loaded provider keys: %s")

    def test_call_litellm_validation(self):
        """Test that call_litellm properly validates inputs"""
        with patch('server.litellm') as mock_litellm:
            # Mock the completion function
            mock_litellm.completion.return_value = {"text": "test response"}

            # Test valid input
            result = call_litellm("openai/gpt-4", ["test message"], 0.7, 100)
            self.assertEqual(result["text"], "test response")

            # Test invalid provider_model format
            result = call_litellm("invalid-model", ["test message"], 0.7, 100)
            self.assertIn("validation error", result["text"])

            # Test missing provider
            result = call_litellm("unknown/gpt-4", ["test message"], 0.7, 100)
            self.assertIn("validation error", result["text"])

            # Test invalid temperature
            result = call_litellm("openai/gpt-4", ["test message"], 1.5, 100)
            self.assertIn("validation error", result["text"])

            # Test invalid max_tokens
            result = call_litellm("openai/gpt-4", ["test message"], 0.7, -10)
            self.assertIn("validation error", result["text"])

    def test_model_servicer_generate_validation(self):
        """Test that ModelServicer.Generate properly validates inputs"""
        servicer = ModelServicer()

        # Create a mock request
        mock_request = MagicMock()
        mock_request.model = "openai/gpt-4"
        mock_request.messages = ["test message"]
        mock_request.temperature = 0.7
        mock_request.max_tokens = 100

        # Create a mock context
        mock_context = MagicMock()

        with patch('server.litellm', None):  # Disable litellm for this test
            with patch('server.logger') as mock_logger:
                # Test valid request
                response = servicer.Generate(mock_request, mock_context)
                self.assertEqual(response.text, "proxy-echo: test message")

                # Test missing model
                mock_request.model = ""
                response = servicer.Generate(mock_request, mock_context)
                self.assertIn("validation error", response.text)

                # Test missing messages
                mock_request.model = "openai/gpt-4"
                mock_request.messages = []
                response = servicer.Generate(mock_request, mock_context)
                self.assertIn("validation error", response.text)

if __name__ == '__main__':
    unittest.main()


