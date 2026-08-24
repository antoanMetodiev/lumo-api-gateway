# Етап 1: Build
FROM eclipse-temurin:21-jdk AS build

WORKDIR /app

# Копиране на Gradle wrapper и скриптове
COPY gradlew .
COPY gradle ./gradle
COPY build.gradle .
COPY settings.gradle .
COPY src ./src

RUN chmod +x gradlew

# Build
RUN ./gradlew bootJar --no-daemon

# Етап 2: Финален образ
FROM eclipse-temurin:21-jdk-jammy

WORKDIR /app

EXPOSE 8080

COPY --from=build /app/build/libs/*.jar app.jar

# -XX:MaxRAMPercentage caps the heap explicitly for Render's free-tier 512MB
# container (the JDK's own container-aware default, 25%, leaves too little
# headroom above it and too much below it to reason about); MaxMetaspaceSize
# bounds Spring's reflection-heavy class metadata so it can't itself balloon
# unbounded; ExitOnOutOfMemoryError makes an OOM a clean container restart
# instead of a wedged, half-alive JVM.
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=70.0", "-XX:MaxMetaspaceSize=96m", "-XX:+ExitOnOutOfMemoryError", "-jar", "app.jar"]