import unittest

from adapter.transform import responses_input_to_messages, responses_to_chat_request, select_model


class DummyServer:
    default_thinking_type = "enabled"
    default_reasoning_effort = "max"


class TransformTests(unittest.TestCase):
    def test_plain_role_content_message_is_preserved(self):
        request = {
            "input": [
                {"role": "user", "content": [{"type": "input_text", "text": "hello"}]},
                {"role": "assistant", "content": [{"type": "output_text", "text": "world"}]},
            ]
        }
        self.assertEqual(
            responses_input_to_messages(request),
            [
                {"role": "user", "content": "hello"},
                {"role": "assistant", "content": "world"},
            ],
        )

    def test_image_base64_routes_to_vision_for_image_url_shape(self):
        request = {
            "input": [
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "image_url",
                            "image_base64": "AAAA",
                            "media_type": "image/png",
                        }
                    ],
                }
            ]
        }
        self.assertEqual(select_model(request, "/v1/responses", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6"), "kimi-k2.6")
        messages = responses_input_to_messages(request)
        image = messages[0]["content"][0]["image_url"]["url"]
        self.assertEqual(image, "data:image/png;base64,AAAA")

    def test_openai_image_url_object_is_not_double_wrapped(self):
        request = {
            "input": [
                {
                    "type": "message",
                    "role": "user",
                    "content": [
                        {
                            "type": "image_url",
                            "image_url": {"url": "data:image/png;base64,BBBB"},
                        }
                    ],
                }
            ]
        }
        messages = responses_input_to_messages(request)
        self.assertEqual(messages[0]["content"][0]["image_url"], {"url": "data:image/png;base64,BBBB"})

    def test_xhigh_maps_to_deepseek_max(self):
        request = {
            "model": "deepseek-v4-pro",
            "input": "hello",
            "reasoning_effort": "xhigh",
        }
        chat = responses_to_chat_request(request, "deepseek-v4-pro", DummyServer())
        self.assertEqual(chat["thinking"], {"type": "enabled"})
        self.assertEqual(chat["reasoning_effort"], "max")

    def test_compact_uses_flash_unless_image_present(self):
        text_request = {"input": "compact me"}
        self.assertEqual(select_model(text_request, "/v1/responses/compact", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6"), "deepseek-v4-flash")
        image_request = {"input": [{"content": [{"type": "input_image", "image_url": "data:image/png;base64,AAAA"}]}]}
        self.assertEqual(select_model(image_request, "/v1/responses/compact", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6"), "kimi-k2.6")


if __name__ == "__main__":
    unittest.main()
