import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

public class ApiKey {

    private static final String SECRET =
            "my-super-secret-key-change-this";

    public static void main(String[] args) {
        String apiKey = generateApiKey(
                "6171ef13-caec-4b01-83e1-b8d0e62f34b4"
        );

        System.out.println(apiKey);
    }

    public static String generateApiKey(String userId) {
        try {

            String payload = String.format(
                    "{\"userId\":\"%s\",\"iat\":%d}",
                    userId,
                    Instant.now().getEpochSecond()
            );

            String encodedPayload = Base64.getUrlEncoder()
                    .withoutPadding()
                    .encodeToString(
                            payload.getBytes(StandardCharsets.UTF_8)
                    );

            String signature = hmacSign(encodedPayload);

            return encodedPayload + "." + signature;

        } catch (Exception e) {
            throw new RuntimeException(
                    "Failed to generate API key",
                    e
            );
        }
    }

    private static String hmacSign(String data) throws Exception {

        Mac mac = Mac.getInstance("HmacSHA256");

        SecretKeySpec secretKeySpec = new SecretKeySpec(
                SECRET.getBytes(StandardCharsets.UTF_8),
                "HmacSHA256"
        );

        mac.init(secretKeySpec);

        byte[] hash = mac.doFinal(
                data.getBytes(StandardCharsets.UTF_8)
        );

        return Base64.getUrlEncoder()
                .withoutPadding()
                .encodeToString(hash);
    }
}