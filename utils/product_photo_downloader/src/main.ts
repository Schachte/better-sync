import axios from "axios";
import * as cheerio from "cheerio";
import * as fs from "fs";
import * as path from "path";
import * as url from "url";
import winston from "winston";
// Import the Google Generative AI SDK
import { GoogleGenerativeAI } from "@google/generative-ai";

import * as dotenv from "dotenv";

// Load environment variables from .env file
dotenv.config();

// Configure logger
const logger = winston.createLogger({
  level: "info",
  format: winston.format.combine(
    winston.format.timestamp(),
    winston.format.printf(
      (info) => `${info.timestamp} - ${info.level}: ${info.message}`
    )
  ),
  transports: [
    new winston.transports.File({ filename: "garmin_photo_download.log" }),
    new winston.transports.Console(),
  ],
});

// Initialize Gemini AI with your API key
// You'll need to provide your Google Gemini API key here
const genAI = new GoogleGenerativeAI(
  process.env.GEMINI_API_KEY || "YOUR_GEMINI_API_KEY"
);

// Interface for watch data
interface GarminWatch {
  series: string;
  product: string;
  model: string;
}

// List of all Garmin watches with music capabilities
const GARMIN_WATCHES: GarminWatch[] = [
  { series: "Forerunner", product: "Forerunner", model: "245_Music" },
  { series: "Forerunner", product: "Forerunner", model: "645_Music" },
  { series: "Forerunner", product: "Forerunner", model: "945" },
  { series: "Forerunner", product: "Forerunner", model: "955" },
  { series: "Forerunner", product: "Forerunner", model: "965" },

  { series: "Fenix", product: "Fenix", model: "5_Plus" },
  { series: "Fenix", product: "Fenix", model: "5_Plus_Sapphire" },
  { series: "Fenix", product: "Fenix", model: "5X_Plus" },
  { series: "Fenix", product: "Fenix", model: "5X_Plus_Sapphire" },
  { series: "Fenix", product: "Fenix", model: "6_Pro" },
  { series: "Fenix", product: "Fenix", model: "6S_Pro" },
  { series: "Fenix", product: "Fenix", model: "6S_Pro_Solar" },
  { series: "Fenix", product: "Fenix", model: "6X_Pro_Solar" },
  { series: "Fenix", product: "Fenix", model: "7" },
  { series: "Fenix", product: "Fenix", model: "7S" },
  { series: "Fenix", product: "Fenix", model: "7X" },

  { series: "Vivoactive", product: "Vivoactive", model: "3_Music" },
  { series: "Vivoactive", product: "Vivoactive", model: "3_Music_Verizon" },
  { series: "Vivoactive", product: "Vivoactive", model: "4" },
  { series: "Vivoactive", product: "Vivoactive", model: "5" },

  { series: "Venu", product: "Venu", model: "" },
  { series: "Venu", product: "Venu_Sq", model: "Music" },
  { series: "Venu", product: "Venu", model: "2" },
  { series: "Venu", product: "Venu", model: "2_Plus" },

  { series: "D2", product: "D2", model: "Delta" },
  { series: "D2", product: "D2", model: "Air" },

  { series: "Enduro", product: "Enduro", model: "2_Music" },

  { series: "MARQ", product: "MARQ", model: "Athlete" },
  { series: "MARQ", product: "MARQ", model: "Commander" },
  { series: "MARQ", product: "MARQ", model: "Adventurer" },
  { series: "MARQ", product: "MARQ", model: "Aviator" },
];

// Directory to save images
const SAVE_DIR = "garmin_watch_images";

// Function to clean filenames
function cleanFilename(filename: string): string {
  return filename.replace(/[\\/*?:"<>|]/g, "_");
}

// Function to search for images using Google search
async function searchGoogleImages(
  query: string,
  numImages: number = 3 // Increased to 3 to have alternatives
): Promise<string[]> {
  const searchUrl = `https://www.google.com/search?q=${encodeURIComponent(
    query
  )}&tbm=isch`;
  const headers = {
    "User-Agent":
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
  };

  logger.info(`Searching for: ${query}`);

  try {
    const response = await axios.get(searchUrl, { headers });
    const $ = cheerio.load(response.data);

    // Find image URLs in the page
    const imageUrls: string[] = [];

    // Extract Google's thumbnail URLs which are more reliable to download
    // Look for thumbnails in Google's image search results
    $("img").each((_, img) => {
      const src = $(img).attr("src");
      if (
        src &&
        (src.startsWith("data:image") || src.includes("googleusercontent.com"))
      ) {
        if (src.startsWith("data:image")) {
          // Skip data URLs for now as they may be too small
          return;
        }

        // These are Google's cached thumbnails which are more reliable to download
        imageUrls.push(src);
      }
    });

    // Backup: If no Google thumbnails found, try regular image URLs
    if (imageUrls.length === 0) {
      $("img").each((_, img) => {
        const src = $(img).attr("src");
        const dataSrc = $(img).attr("data-src");

        if (src && src.startsWith("http")) {
          imageUrls.push(src);
        }
        if (dataSrc && dataSrc.startsWith("http")) {
          imageUrls.push(dataSrc);
        }
      });
    }

    // Look for JSON data in scripts that might contain image information
    const scriptRegex =
      /\["(https?:\/\/[^"]+\.(?:jpg|jpeg|png|webp|gif)[^"]*)",\d+,\d+\]/g;
    $("script").each((_, script) => {
      const scriptContent = $(script).html();
      if (scriptContent) {
        const matches = scriptContent.match(scriptRegex);
        if (matches) {
          matches.forEach((match) => {
            try {
              // Extract URL from the matched pattern
              const urlMatch = match.match(/"(https?:\/\/[^"]+)"/);
              if (urlMatch && urlMatch[1]) {
                imageUrls.push(urlMatch[1]);
              }
            } catch (e) {
              // Skip malformed matches
            }
          });
        }
      }
    });

    // Filter out duplicates
    const uniqueUrls = [...new Set(imageUrls)];

    // Return the specified number of image URLs
    return uniqueUrls.slice(0, numImages);
  } catch (error) {
    logger.error(`Error searching for images: ${error}`);
    return [];
  }
}

// Function to download an image
async function downloadImage(
  imageUrl: string,
  savePath: string
): Promise<boolean> {
  try {
    // Handle URL parsing safely
    let finalSavePath = savePath;
    let ext = "";

    try {
      const parsedUrl = new URL(imageUrl);
      ext = path.extname(parsedUrl.pathname || "").toLowerCase();
    } catch (urlError) {
      logger.warn(`Invalid URL format, using default extension: ${urlError}`);
      ext = ".jpg"; // Default extension
    }

    const validExtensions = [".jpg", ".jpeg", ".png", ".webp", ".gif"];

    // Add extension if needed
    if (!validExtensions.includes(ext)) {
      finalSavePath += ".jpg"; // Default to jpg if no extension found
    } else if (!savePath.toLowerCase().endsWith(ext)) {
      finalSavePath += ext;
    }

    const headers = {
      "User-Agent":
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
      Referer: "https://www.google.com/",
      // Add more headers to help bypass restrictions
      Accept: "image/avif,image/webp,image/apng,image/*,*/*;q=0.8",
      "Accept-Language": "en-US,en;q=0.9",
      "Sec-Fetch-Dest": "image",
      "Sec-Fetch-Mode": "no-cors",
      "Sec-Fetch-Site": "cross-site",
    };

    const response = await axios({
      method: "get",
      url: imageUrl,
      responseType: "stream",
      headers,
      timeout: 10000, // 10 second timeout
      maxRedirects: 5,
    });

    const writer = fs.createWriteStream(finalSavePath);

    response.data.pipe(writer);

    return new Promise<boolean>((resolve, reject) => {
      writer.on("finish", () => {
        logger.info(`Downloaded image to: ${finalSavePath}`);
        resolve(true);
      });
      writer.on("error", (err) => {
        logger.error(`Error writing file: ${err}`);
        reject(false);
      });
    });
  } catch (error) {
    logger.error(`Error downloading image: ${error}`);
    return false;
  }
}

// Function to validate the image using Gemini AI
async function validateImageWithGemini(
  imagePath: string,
  watchModel: string
): Promise<boolean> {
  try {
    // Check if file exists
    if (!fs.existsSync(imagePath)) {
      logger.error(`Image file does not exist: ${imagePath}`);
      return false;
    }

    // Read the image file as base64
    const imageFile = fs.readFileSync(imagePath);
    const base64Image = imageFile.toString("base64");

    // Initialize the Gemini Pro Vision model
    const model = genAI.getGenerativeModel({ model: "gemini-1.5-pro" });

    // Prompt to evaluate if the image shows a front-facing watch
    const prompt = `
      Please analyze this image and tell me if it contains ONLY a front-facing watch (specifically a Garmin ${watchModel}) with nothing else in the frame.
      
      Requirements:
      1. The image must show ONLY the watch face from the front
      2. The watch should not be shown from the back
      3. There should not be any other objects, people, or text in the image
      4. The watch should be the clear and central subject
      5. Photo should be high quality and larger than 300x300 pixels
      6. Ensure than the photo is not grainy, blurry or low resolution at all. Should be 4k quality
      
      Answer with ONLY 'YES' if ALL requirements are met, or 'NO' if ANY requirement is not met.
    `;

    // Create content parts for the prompt and image
    const imageParts = [
      { text: prompt },
      {
        inlineData: {
          data: base64Image,
          mimeType: "image/jpeg", // Adjust based on your image type
        },
      },
    ];

    // Generate content
    const result = await model.generateContent({
      contents: [{ role: "user", parts: imageParts }],
    });

    const response = result.response;
    const text = response.text().trim().toUpperCase();

    // Check if the AI response indicates this is a valid watch image
    const isValid = text.includes("YES");

    logger.info(
      `Gemini AI validation for ${watchModel}: ${isValid ? "PASSED" : "FAILED"}`
    );

    return isValid;
  } catch (error) {
    logger.error(`Error validating image with Gemini AI: ${error}`);
    return false;
  }
}

// Function to check if an image for a watch model already exists
function watchImageExists(baseFilePath: string): boolean {
  const validExtensions = [".jpg", ".jpeg", ".png", ".webp", ".gif"];

  // Check if any file with the baseFilePath and any valid extension exists
  for (const ext of validExtensions) {
    const potentialPath = baseFilePath + ext;
    if (fs.existsSync(potentialPath)) {
      logger.info(`Image already exists at ${potentialPath}`);
      return true;
    }
  }

  return false;
}

// Main function to find and download images for all watches
async function downloadAllWatchImages(): Promise<void> {
  let successfulDownloads = 0;
  let failedDownloads = 0;
  let skippedDownloads = 0;

  // Create directory if it doesn't exist
  if (!fs.existsSync(SAVE_DIR)) {
    fs.mkdirSync(SAVE_DIR, { recursive: true });
  }

  for (const watch of GARMIN_WATCHES) {
    const { series, product, model } = watch;

    // Create search query - improved for more accurate results
    const fullModelName = model
      ? `Garmin ${product} ${model}`
      : `Garmin ${product}`;

    const searchQuery = `${fullModelName} watch product photo official front facing high resolution`;

    // Create filename
    const filename = model
      ? `${series}_${product}_${model}`
      : `${series}_${product}`;

    const cleanedFilename = cleanFilename(filename);
    const baseFilePath = path.join(SAVE_DIR, cleanedFilename);

    // Check if image already exists for this watch model
    if (watchImageExists(baseFilePath)) {
      logger.info(`Skipping ${fullModelName} - image already exists`);
      skippedDownloads++;
      continue;
    }

    // Search for images
    const imageUrls = await searchGoogleImages(searchQuery, 3); // Get up to 3 images

    if (imageUrls.length === 0) {
      logger.warn(`No images found for ${fullModelName}`);
      failedDownloads++;
      continue;
    }

    // Try to download and validate up to 3 images
    let success = false;
    for (let i = 0; i < Math.min(3, imageUrls.length); i++) {
      const url = imageUrls[i];
      const tempSavePath = `${baseFilePath}_temp${i}`;

      try {
        // Download the image
        const downloadSuccess = await downloadImage(url, tempSavePath);
        if (!downloadSuccess) {
          logger.warn(`Failed to download image from ${url}`);
          continue;
        }

        // Determine the actual file path with extension - safely
        let actualFilePath = tempSavePath;
        let ext = "";

        try {
          const parsedUrl = new URL(url);
          ext = path.extname(parsedUrl.pathname || "").toLowerCase();
        } catch (urlError) {
          logger.warn(
            `Invalid URL format for path detection, using default extension: ${urlError}`
          );
          ext = ".jpg"; // Default extension
        }

        const validExtensions = [".jpg", ".jpeg", ".png", ".webp", ".gif"];

        if (!validExtensions.includes(ext)) {
          actualFilePath += ".jpg"; // Default to jpg if no extension found
        } else if (!tempSavePath.toLowerCase().endsWith(ext)) {
          actualFilePath += ext;
        }

        // Validate the image using Gemini AI
        const isValid = await validateImageWithGemini(
          actualFilePath,
          fullModelName
        );

        if (isValid) {
          // Rename the file to the final name if it's valid
          const finalPath = baseFilePath + path.extname(actualFilePath);
          fs.renameSync(actualFilePath, finalPath);
          logger.info(`Valid image found and saved as ${finalPath}`);
          success = true;
          break;
        } else {
          // Delete the invalid image
          logger.info(`Deleting invalid image ${actualFilePath}`);
          fs.unlinkSync(actualFilePath);
        }
      } catch (error) {
        logger.error(`Error processing ${url}: ${error}`);
        // Clean up temp file if it exists
        try {
          if (fs.existsSync(tempSavePath)) {
            fs.unlinkSync(tempSavePath);
          }
        } catch (cleanupError) {
          logger.error(`Error cleaning up temp file: ${cleanupError}`);
        }
      }

      // Sleep to avoid overloading servers
      await new Promise((resolve) => setTimeout(resolve, 2000));
    }

    if (success) {
      successfulDownloads++;
    } else {
      logger.warn(
        `Failed to find a valid image for ${fullModelName} after trying 3 images`
      );
      failedDownloads++;
    }

    // Sleep to avoid overloading the server
    await new Promise((resolve) => setTimeout(resolve, 3000));
  }

  logger.info(
    `Download complete. Successful: ${successfulDownloads}, Failed: ${failedDownloads}, Skipped: ${skippedDownloads}`
  );
}

// Run the main function
(async () => {
  logger.info("Starting Garmin watch image download with Gemini AI validation");

  // Check if Gemini API key is available - fixed logic
  if (
    !process.env.GEMINI_API_KEY ||
    process.env.GEMINI_API_KEY === "YOUR_GEMINI_API_KEY"
  ) {
    logger.error(
      "Gemini API key not provided. Please set the GEMINI_API_KEY environment variable or update the script."
    );
    return;
  }

  try {
    await downloadAllWatchImages();
  } catch (error) {
    logger.error(`Script error: ${error}`);
  }
  logger.info("Script execution completed");
})();
