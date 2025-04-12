FROM nginx:latest
COPY ./index.html /usr/share/nginx/html/index.html

# # Step 1: Build the React app to /app/dist
# FROM node:18-alpine AS build

# WORKDIR /app

# COPY package*.json ./
# RUN npm install

# COPY . .
# RUN npm run build

# # Step 2. NGINX
# FROM nginx:latest

# # Copy build files from /app/dist to nginx html folder
# COPY --from=build /app/dist /usr/share/nginx/html/
