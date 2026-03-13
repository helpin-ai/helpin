# Use the official maven/Java 18 image to create a build artifact
FROM maven:3.8.7-openjdk-18 as builder

# Set the working directory
WORKDIR /usr/src/app

# Copy the pom.xml file into the working directory
COPY pom.xml .

# Download all required dependencies into one layer
RUN mvn -B dependency:go-offline

# Copy your source code into the image
RUN ls -al
COPY src ./src

# Package the application
RUN mvn package

# Use OpenJDK JRE 20.0.1 base image
FROM amazoncorretto:20.0.1
ENV TZ=UTC
# Set the working directory
WORKDIR /app

ARG APP_RELEASE_VERSION

# Copy JAR from the builder stage
COPY --from=builder /usr/src/app/target/eventspipeline-1.0-SNAPSHOT-jar-with-dependencies.jar app.jar


# # Make sure that the application is accessible from outside the container
# EXPOSE 8080

# Run the application
ENTRYPOINT ["java", "-jar", "app.jar"]
