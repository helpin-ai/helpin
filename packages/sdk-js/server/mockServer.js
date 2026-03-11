const express = require('express');
const cors = require('cors');
const bodyParser = require('body-parser');
const morgan = require('morgan');
const winston = require('winston');
const colors = require('colors'); // Ensure you're using 'colors' and not 'colors/safe'
const fs = require('fs');

const app = express();
const port = 3000;
const logFilename = 'mock-server.log';

// Define an array of properties to highlight with their respective colors
const propertiesToHighlight = [
  { name: 'event_type', color: colors.green },
  { name: 'user_id', color: colors.blue },
  { name: 'email', color: colors.cyan },
  // Add more properties as needed
];

// Function to clear the log file
function clearLogFile() {
  fs.writeFileSync(logFilename, '');
  console.log(colors.yellow(`Cleared ${logFilename}`));
}

// Clear the log file on server start
clearLogFile();

// Define separate formats for console and file

// Console Format: Colorize specific properties
const consoleFormat = winston.format.combine(
  winston.format.timestamp(),
  winston.format.printf(({ level, message, timestamp }) => {
    let coloredMessage = message;

    // Iterate over each property to highlight
    propertiesToHighlight.forEach(({ name, color }) => {
      // Create a dynamic regex for each property
      const regex = new RegExp(`("${name}"\\s*:\\s*)"([^"]+)"`, 'g');
      // Replace the matched value with colored value
      coloredMessage = coloredMessage.replace(regex, (match, p1, p2) => {
        return `${p1}${color(`"${p2}"`)}`;
      });
    });

    return `${colors.gray(timestamp)} [${colors.blue(level)}]: ${coloredMessage}`;
  }),
);

// File Format: Pretty-print JSON with indentation
const fileFormat = winston.format.combine(
  winston.format.timestamp(),
  winston.format.json({ space: 2 }),
  // Alternatively, use the custom printf format as shown in Option B below
);

// Configure Winston logger with separate formats for console and file
const logger = winston.createLogger({
  level: 'info',
  transports: [
    new winston.transports.Console({
      format: consoleFormat,
    }),
    new winston.transports.File({
      filename: logFilename,
      format: fileFormat,
    }),
  ],
});

// Middleware
app.use(cors());
app.use(bodyParser.json());
app.use(morgan('dev'));

// Mock endpoint for event tracking
app.post('/api/v1/event', (req, res) => {
  // Serialize the request body to JSON with indentation
  const jsonString = JSON.stringify(req.body, null, 2);
  logger.info(`Received event:\n${jsonString}`);

  // Simulate processing delay
  setTimeout(() => {
    res
      .status(200)
      .json({ success: true, message: 'Event received and processed' });
  }, 200);
});

app.post('/api/v1/s2s/event', (req, res) => {
  // Serialize the request body to JSON with indentation
  const jsonString = JSON.stringify(req.body, null, 2);
  logger.info(`Received server-side event:\n${jsonString}`);

  // Simulate processing delay
  setTimeout(() => {
    res
      .status(200)
      .json({
        success: true,
        message: 'Server Side Event received and processed',
      });
  }, 200);
});

// Error handling middleware
app.use((err, req, res, next) => {
  logger.error(`Error: ${err.message}`);
  res.status(500).json({ error: 'Internal server error' });
});

// Start the server
app.listen(port, () => {
  logger.info(`Mock server running at http://localhost:${port}`);
});

// Graceful shutdown
process.on('SIGTERM', () => {
  logger.info('SIGTERM signal received: closing HTTP server');
  app.close(() => {
    logger.info('HTTP server closed');
  });
});
