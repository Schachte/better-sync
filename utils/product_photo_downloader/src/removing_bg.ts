import * as fs from "fs";
import * as path from "path";
import * as dotenv from "dotenv";
import axios from "axios";
import { promisify } from "util";
import FormData from "form-data";

// Load environment variables from .env file
// Try to load from root directory first, then current directory
const rootEnvPath = path.resolve(__dirname, "..", "..", ".env");
const localEnvPath = path.resolve(__dirname, ".env");

if (fs.existsSync(rootEnvPath)) {
  dotenv.config({ path: rootEnvPath });
} else if (fs.existsSync(localEnvPath)) {
  dotenv.config({ path: localEnvPath });
} else {
  dotenv.config();
}

const apiKey = process.env.REMOVE_BG_API_KEY;

if (!apiKey) {
  console.error("Error: REMOVE_BG_API_KEY environment variable not found");
  console.error(
    "Please create a .env file with REMOVE_BG_API_KEY=your_api_key in the root directory or in product_photo_downloader"
  );
  process.exit(1);
}

const readdir = promisify(fs.readdir);
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);
const stat = promisify(fs.stat);

// Set up paths based on project structure
const PROJECT_ROOT = path.resolve(__dirname, "..", ".."); // Go up two levels from script
const DOWNLOADER_DIR = path.resolve(PROJECT_ROOT, "product_photo_downloader");
const LOG_DIR = path.resolve(PROJECT_ROOT, "logs");

// Create log directory if it doesn't exist
if (!fs.existsSync(LOG_DIR)) {
  fs.mkdirSync(LOG_DIR, { recursive: true });
}

// Log file path
const LOG_FILE = path.join(LOG_DIR, "bg_removal.log");

// Source and output directories
const sourceDir = path.join(DOWNLOADER_DIR, "garmin_watch_images");
const outputDir = path.join(DOWNLOADER_DIR, "garmin_watch_images_nobg");

// Supported image formats
const supportedFormats = [".png", ".jpg", ".jpeg"];

/**
 * Log a message to console and file
 * @param message The message to log
 * @param isError Whether this is an error message
 */
function logMessage(message: string, isError: boolean = false): void {
  const timestamp = new Date().toISOString();
  const logEntry = `[${timestamp}] ${message}`;

  if (isError) {
    console.error(logEntry);
  } else {
    console.log(logEntry);
  }

  // Append to log file
  fs.appendFileSync(LOG_FILE, logEntry + "\n");
}

// Create output directory if it doesn't exist
if (!fs.existsSync(outputDir)) {
  fs.mkdirSync(outputDir, { recursive: true });
  logMessage(`Created output directory: ${outputDir}`);
}

/**
 * Removes background from an image using remove.bg API
 * @param filePath Path to the image file
 * @returns Promise with the processed image buffer
 */
async function removeBackground(filePath: string): Promise<Buffer> {
  const imageBuffer = await readFile(filePath);

  try {
    // Create form data with the correct field name 'image_file'
    const formData = new FormData();
    formData.append("image_file", imageBuffer, {
      filename: path.basename(filePath),
      contentType: `image/${path.extname(filePath).slice(1)}`, // e.g., image/jpeg
    });

    const response = await axios({
      method: "post",
      url: "https://api.remove.bg/v1.0/removebg",
      data: formData,
      responseType: "arraybuffer",
      headers: {
        "X-Api-Key": apiKey,
        ...formData.getHeaders(),
      },
    });

    return Buffer.from(response.data);
  } catch (error) {
    if (axios.isAxiosError(error) && error.response) {
      const errMsg = `API Error (${error.response.status}): ${error.response.statusText}`;
      logMessage(errMsg, true);

      if (error.response.data) {
        try {
          // For error responses, the API usually returns JSON
          const errorData = JSON.parse(
            Buffer.from(error.response.data).toString()
          );
          logMessage(
            `Error details: ${JSON.stringify(errorData.errors)}`,
            true
          );
        } catch {
          logMessage(
            `Error response: ${Buffer.from(error.response.data).toString()}`,
            true
          );
        }
      }
    } else {
      logMessage(`Error removing background: ${error}`, true);
    }
    throw error;
  }
}

/**
 * Process all images in a directory
 */
async function processDirectory() {
  logMessage(`Starting background removal process`);
  logMessage(`Source directory: ${sourceDir}`);
  logMessage(`Output directory: ${outputDir}`);

  try {
    // Check if source directory exists
    if (!fs.existsSync(sourceDir)) {
      logMessage(`Error: Source directory does not exist: ${sourceDir}`, true);
      process.exit(1);
    }

    // Get all files in the source directory
    const files = await readdir(sourceDir);

    // Get existing processed files (to skip reprocessing)
    const existingOutputFiles = fs.existsSync(outputDir)
      ? new Set((await readdir(outputDir)).map((file) => file.toLowerCase()))
      : new Set();

    // Count of images to process
    const imageFiles = files.filter((file) => {
      const ext = path.extname(file).toLowerCase();
      return supportedFormats.includes(ext);
    });

    logMessage(`Found ${imageFiles.length} total images in source directory`);

    // Track stats
    let processed = 0;
    let skipped = 0;
    let failed = 0;

    // Process each image
    for (const [index, file] of imageFiles.entries()) {
      const filePath = path.join(sourceDir, file);
      const fileInfo = await stat(filePath);

      // Skip directories
      if (fileInfo.isDirectory()) {
        continue;
      }

      const ext = path.extname(file).toLowerCase();

      // Check if it's a supported image format
      if (supportedFormats.includes(ext)) {
        // Generate output filename
        const outputExt = ".png"; // remove.bg returns PNG format
        const outputFilename = path.basename(file, ext) + "_nobg" + outputExt;
        const outputPath = path.join(outputDir, outputFilename);

        // Check if file already exists in output directory and not in force mode
        if (
          !forceAll &&
          existingOutputFiles.has(outputFilename.toLowerCase())
        ) {
          logMessage(
            `[${index + 1}/${
              imageFiles.length
            }] Skipping: ${file} (already processed)`
          );
          skipped++;
          continue;
        }

        logMessage(`[${index + 1}/${imageFiles.length}] Processing: ${file}`);

        try {
          // Remove background and save result
          const processedImage = await removeBackground(filePath);
          await writeFile(outputPath, processedImage);

          logMessage(`  ✓ Saved to: ${outputPath}`);
          processed++;
        } catch (error) {
          logMessage(`  ✗ Failed to process ${file}`, true);
          failed++;
        }
      }
    }

    // Print summary
    logMessage(`\nProcessing complete:`);
    logMessage(`  ✓ Successfully processed: ${processed}`);
    logMessage(`  ↷ Skipped (already exist): ${skipped}`);
    logMessage(`  ✗ Failed to process: ${failed}`);
    logMessage(`  Total images handled: ${processed + skipped + failed}`);
  } catch (error) {
    logMessage(`Error processing directory: ${error}`, true);
  }
}

// Parse command line arguments
const args = process.argv.slice(2);
const forceAll = args.includes("--force") || args.includes("-f");

if (forceAll) {
  logMessage(
    "Force mode enabled: will reprocess all images regardless of existing output files"
  );
}

// Run the main function
processDirectory().catch((error) =>
  logMessage(`Unhandled error: ${error}`, true)
);
