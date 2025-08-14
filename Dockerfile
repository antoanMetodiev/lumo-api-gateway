# Етап 1: Build
FROM gradle:8.3-jdk21 AS build

WORKDIR /app

# Копиране на Gradle wrapper и конфигурация
COPY gradle gradle
COPY gradlew .
COPY build.gradle.kts .
COPY settings.gradle.kts .

# Кеширане на зависимостите
RUN ./gradlew --no-daemon buildEnvironment

# Копиране на останалите файлове
COPY . .

# Създаване на JAR
RUN ./gradlew bootJar --no-daemon

# Етап 2: Финален образ
FROM eclipse-temurin:21-jdk-jammy AS runtime

WORKDIR /app

# Излагане на порта
EXPOSE 8080

# Копиране само на финалния JAR
COPY --from=build /app/build/libs/*.jar app.jar

# Стартиране на приложението
ENTRYPOINT ["java", "-jar", "app.jar"]