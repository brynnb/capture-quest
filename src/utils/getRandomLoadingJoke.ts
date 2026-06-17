export class LoadingJokeUtil {
  private static jokes: string[] = [
    "Waking up Snorlax with a Poké Flute",
    "Restocking Poké Balls at the shop",
    "Checking under every truck for Mew",
    "Polishing Gym Badges",
    "Healing your team at the Pokémon Center",
    "Avoiding Youngster Joey's phone calls",
    "Searching the tall grass for Shinies",
    "Charging the Rotom Phone",
    "Organizing PC boxes (by type, of course)",
    "Consulting Professor Oak",
    "Teaching Magikarp how to Splash",
    "Calibrating the Master Ball",
    "Surviving yet another Zubat encounter",
    "Incubating mystery eggs",
    "Winning the Indigo Plateau",
    "Crafting the perfect sandwich",
    "Trading with a friend (don't forget the Everstone!)",
    "Dodging Team Rocket's latest trap",
    "Consulting the Type Matchup chart",
    "Feeding Poffins to your favorites",
    "Preparing for the Elite Four",
    "Exploring the depths of Mt. Moon",
    "Waiting for your Eevee to evolve",
    "Stocking up on Full Restores",
    "Setting off on a new adventure",
    "Catching 'em all... eventually",
    "Studying ancient ruins in Johto",
    "Riding the Cycling Road",
    "Equipping the Exp. Share",
    "Talking to every NPC in town",
    "Entering the Safari Zone",
    "Checking your Town Map",
    "Listening to the Poké Flute",
    "Finding the HM for Strength",
    "Dodging a hyper-realistic Gengar",
    "Trying to fit a Wailord in a Poké Ball",
    "Collecting Soothe Bells",
    "Preparing your move-set",
    "Becoming a Pokémon Master"
  ];

  public static getRandomLoadingJoke(): string {
    const randomIndex = Math.floor(Math.random() * this.jokes.length);
    return this.jokes[randomIndex];
  }
}
