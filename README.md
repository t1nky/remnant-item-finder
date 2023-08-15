# ReFinder - `Remnant 2` item finder

ReFinder is a CLI tool that reads your save file and lets you see what items, events, and rewards you have on the map.

<img width="379" alt="image" src="https://github.com/t1nky/remnant-item-finder/assets/1833969/5b2f52bf-97fa-484e-937e-5023275bda5c">

### Usage

TODO: Instructions on how to use the application.

### Prerequisites

- Go 1.21 or later

### Installation

Clone this repository:

```bash
git clone https://github.com/t1nky/remnant-item-finder.git
```

Move to the project directory:

```bash
cd remnant-item-finder
```

Then build the project:

```bash
go build
```

### TODO

- Find spawned actors (vendors/NPCs)
- Autodetect if main story or adventure is active
  - Show main story if it's active
- Use some UI framework like Wails to make it fancy and allow interactivity
  - Allow changing the save file path
  - Allow manual character selection
  - Allow manual world selection (main story/adventure)

### Contributing

We appreciate all your contributions. If you're interested in contributing, please take a look at our CONTRIBUTING.md for details on our code of conduct and the process for submitting pull requests.

### License

This project is licensed under the MIT License. Please have a look at LICENSE.md for more details.
